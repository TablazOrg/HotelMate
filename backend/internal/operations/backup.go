package operations

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

const recoverySchemaVersion = "hotelmate.recovery-set/v1"

var recoverySetID = regexp.MustCompile(`^hotelmate-[0-9]{8}T[0-9]{6}Z$`)

type recoveryArtifact struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type recoverySet struct {
	SchemaVersion  string                     `json:"schemaVersion"`
	ID             string                     `json:"id"`
	CreatedAt      time.Time                  `json:"createdAt"`
	Environment    string                     `json:"environment"`
	ReleaseVersion string                     `json:"releaseVersion"`
	Database       recoveryArtifact           `json:"database"`
	Uploads        *recoveryArtifact          `json:"uploads,omitempty"`
	Migrations     []database.MigrationStatus `json:"migrations"`
	OffHost        bool                       `json:"offHost"`
	ResticSnapshot string                     `json:"resticSnapshot,omitempty"`
	ManifestFile   string                     `json:"-"`
}

func (a *App) backup(ctx context.Context, config resolvedConfig, path []string, options cliOptions) (any, error) {
	if len(path) != 2 {
		return nil, failure(ExitInvalid, "usage: hotelmate backup create|list|verify|restore|drill", nil)
	}
	switch path[1] {
	case "create":
		if !options.yes {
			return nil, failure(ExitPrecondition, fmt.Sprintf("backup of %s requires --yes", config.Environment), nil)
		}
		return a.createBackup(ctx, config)
	case "list":
		return listBackups(config.BackupDir)
	case "verify":
		manifest, err := resolveRecoverySet(config)
		if err != nil {
			return nil, err
		}
		return a.verifyBackup(ctx, config, manifest)
	case "restore":
		if !options.yes {
			return nil, failure(ExitPrecondition, fmt.Sprintf("restore into %s requires --yes", config.Environment), nil)
		}
		manifest, err := resolveRecoverySet(config)
		if err != nil {
			return nil, err
		}
		releaseLock, err := acquireEnvironmentLock(config.EvidenceDir, config.Environment)
		if err != nil {
			return nil, err
		}
		defer releaseLock()
		return a.restoreBackup(ctx, config, manifest)
	case "drill":
		if !options.yes {
			return nil, failure(ExitPrecondition, fmt.Sprintf("recovery drill in %s requires --yes", config.Environment), nil)
		}
		return a.recoveryDrill(ctx, config)
	default:
		return nil, failure(ExitInvalid, "usage: hotelmate backup create|list|verify|restore|drill", nil)
	}
}

func (a *App) createBackup(ctx context.Context, config resolvedConfig) (any, error) {
	if !filepath.IsAbs(config.BackupDir) {
		return nil, failure(ExitInvalid, "backup directory must be absolute", nil)
	}
	if config.PostgresDriver == "direct" {
		if _, err := a.LookPath("pg_dump"); err != nil {
			return nil, failure(ExitPrecondition, "pg_dump is required", err)
		}
	}
	if err := os.MkdirAll(config.BackupDir, 0o700); err != nil {
		return nil, failure(ExitPrecondition, "create backup directory failed", err)
	}
	if err := os.Chmod(config.BackupDir, 0o700); err != nil {
		return nil, failure(ExitPrecondition, "protect backup directory failed", err)
	}
	createdAt := a.Now().UTC()
	id := "hotelmate-" + createdAt.Format("20060102T150405Z")
	dumpPath := filepath.Join(config.BackupDir, id+".dump")
	temporaryDump := dumpPath + ".partial"
	_ = os.Remove(temporaryDump)
	var diagnostics bytes.Buffer
	err := a.createPostgresDump(ctx, config, temporaryDump, &diagnostics)
	if err != nil {
		_ = os.Remove(temporaryDump)
		return nil, failure(ExitCommand, "PostgreSQL backup failed", sanitizedToolError(err, diagnostics.String()))
	}
	if err := os.Chmod(temporaryDump, 0o600); err != nil {
		_ = os.Remove(temporaryDump)
		return nil, failure(ExitCommand, "protect database artifact failed", err)
	}
	if err := os.Rename(temporaryDump, dumpPath); err != nil {
		_ = os.Remove(temporaryDump)
		return nil, failure(ExitCommand, "finalize database artifact failed", err)
	}
	databaseArtifact, err := inspectArtifact(dumpPath)
	if err != nil {
		return nil, failure(ExitVerification, "database artifact inspection failed", err)
	}
	if databaseArtifact.Bytes < 1024 {
		return nil, failure(ExitVerification, "database artifact is unexpectedly small", nil)
	}
	if _, err := a.verifyPostgresCatalog(ctx, config, dumpPath); err != nil {
		return nil, err
	}

	var uploadsArtifact *recoveryArtifact
	if info, err := os.Stat(config.UploadsDir); err == nil && info.IsDir() {
		uploadsPath := filepath.Join(config.BackupDir, id+"-uploads.tar.gz")
		files, err := createUploadsArchive(config.UploadsDir, uploadsPath)
		if err != nil {
			return nil, failure(ExitCommand, "private upload backup failed", err)
		}
		if files > 0 {
			artifact, err := inspectArtifact(uploadsPath)
			if err != nil {
				return nil, failure(ExitVerification, "upload artifact inspection failed", err)
			}
			uploadsArtifact = &artifact
		} else {
			_ = os.Remove(uploadsPath)
		}
	}

	databaseContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	db, err := database.Open(databaseContext, config.DatabaseURL)
	cancel()
	if err != nil {
		return nil, failure(ExitVerification, "read migration evidence failed", err)
	}
	migrations, err := database.MigrationStatuses(db)
	_ = database.Close(db)
	if err != nil {
		return nil, failure(ExitVerification, "read migration evidence failed", err)
	}
	manifest := recoverySet{
		SchemaVersion: recoverySchemaVersion, ID: id, CreatedAt: createdAt, Environment: config.Environment,
		ReleaseVersion: config.Application.APIVersion, Database: databaseArtifact, Uploads: uploadsArtifact, Migrations: migrations,
	}
	manifestPath := filepath.Join(config.BackupDir, id+".json")
	manifest.ManifestFile = manifestPath
	if err := writeJSONAtomic(manifestPath, manifest, 0o600); err != nil {
		return nil, failure(ExitCommand, "write recovery manifest failed", err)
	}
	if config.ResticRepository != "" {
		snapshot, err := a.sendRecoverySetOffHost(ctx, config, manifest)
		if err != nil {
			return map[string]any{"manifest": manifestPath, "recoverySet": manifest}, err
		}
		manifest.OffHost = true
		manifest.ResticSnapshot = snapshot
		if err := writeJSONAtomic(manifestPath, manifest, 0o600); err != nil {
			return nil, failure(ExitCommand, "record off-host snapshot failed", err)
		}
		if err := a.sendRecoveryManifestOffHost(ctx, config, manifest); err != nil {
			return map[string]any{"manifest": manifestPath, "recoverySet": manifest}, err
		}
		if err := a.enforceResticRetention(ctx, config); err != nil {
			return map[string]any{"manifest": manifestPath, "recoverySet": manifest}, err
		}
	}
	return map[string]any{"manifest": manifestPath, "recoverySet": manifest, "verified": true}, nil
}

func (a *App) sendRecoverySetOffHost(ctx context.Context, config resolvedConfig, manifest recoverySet) (string, error) {
	if _, err := a.LookPath("restic"); err != nil {
		return "", failure(ExitPrecondition, "restic is required for off-host backup", err)
	}
	files := []string{filepath.Join(filepath.Dir(manifest.ManifestFile), manifest.Database.File)}
	if manifest.Uploads != nil {
		files = append(files, filepath.Join(filepath.Dir(manifest.ManifestFile), manifest.Uploads.File))
	}
	env := []string{"RESTIC_REPOSITORY=" + config.ResticRepository, "RESTIC_PASSWORD=" + config.ResticPassword}
	var stdout, stderr bytes.Buffer
	args := append([]string{"backup", "--json", "--tag", "hotelmate", "--tag", "environment:" + config.Environment, "--tag", "recovery:" + manifest.ID}, files...)
	if err := a.Executor.Run(ctx, "restic", args, env, nil, &stdout, &stderr); err != nil {
		return "", failure(ExitVerification, "encrypted off-host transfer failed", sanitizedToolError(err, stderr.String()))
	}
	snapshot := resticSnapshotID(stdout.Bytes())
	if snapshot == "" {
		return "", failure(ExitVerification, "restic did not report an off-host snapshot identity", nil)
	}
	return snapshot, nil
}

func (a *App) sendRecoveryManifestOffHost(ctx context.Context, config resolvedConfig, manifest recoverySet) error {
	env := []string{"RESTIC_REPOSITORY=" + config.ResticRepository, "RESTIC_PASSWORD=" + config.ResticPassword}
	var stderr bytes.Buffer
	args := []string{"backup", "--tag", "hotelmate", "--tag", "environment:" + config.Environment, "--tag", "recovery:" + manifest.ID, manifest.ManifestFile}
	if err := a.Executor.Run(ctx, "restic", args, env, nil, ioDiscard{}, &stderr); err != nil {
		return failure(ExitVerification, "off-host recovery manifest transfer failed", sanitizedToolError(err, stderr.String()))
	}
	return nil
}

func (a *App) enforceResticRetention(ctx context.Context, config resolvedConfig) error {
	env := []string{"RESTIC_REPOSITORY=" + config.ResticRepository, "RESTIC_PASSWORD=" + config.ResticPassword}
	var stderr bytes.Buffer
	if err := a.Executor.Run(ctx, "restic", []string{"check"}, env, nil, ioDiscard{}, &stderr); err != nil {
		return failure(ExitVerification, "off-host repository verification failed", sanitizedToolError(err, stderr.String()))
	}
	forgetArgs := []string{"forget", "--prune", "--tag", "hotelmate", "--tag", "environment:" + config.Environment, "--keep-daily", strconv.Itoa(config.ResticKeepDaily), "--keep-weekly", strconv.Itoa(config.ResticKeepWeekly)}
	stderr.Reset()
	if err := a.Executor.Run(ctx, "restic", forgetArgs, env, nil, ioDiscard{}, &stderr); err != nil {
		return failure(ExitVerification, "off-host retention enforcement failed", sanitizedToolError(err, stderr.String()))
	}
	return nil
}

func resticSnapshotID(output []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var item map[string]any
		if err := decoder.Decode(&item); err != nil {
			break
		}
		if item["message_type"] == "summary" {
			if value, ok := item["snapshot_id"].(string); ok {
				return value
			}
		}
	}
	return ""
}

func listBackups(directory string) (any, error) {
	entries, err := filepath.Glob(filepath.Join(directory, "hotelmate-*.json"))
	if err != nil {
		return nil, failure(ExitCommand, "read backup catalog failed", err)
	}
	sets := make([]recoverySet, 0, len(entries))
	for _, entry := range entries {
		manifest, err := readRecoverySet(entry)
		if err != nil {
			return nil, failure(ExitVerification, "backup catalog contains an invalid manifest", err)
		}
		sets = append(sets, manifest)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].CreatedAt.After(sets[j].CreatedAt) })
	return map[string]any{"directory": directory, "count": len(sets), "recoverySets": sets}, nil
}

func resolveRecoverySet(config resolvedConfig) (recoverySet, error) {
	path := config.ManifestFile
	if path == "" {
		data, err := listBackups(config.BackupDir)
		if err != nil {
			return recoverySet{}, err
		}
		sets := data.(map[string]any)["recoverySets"].([]recoverySet)
		if len(sets) == 0 {
			return recoverySet{}, failure(ExitPrecondition, "no recovery sets found; pass --manifest", nil)
		}
		return sets[0], nil
	}
	if strings.HasSuffix(path, ".dump") {
		candidate := strings.TrimSuffix(path, ".dump") + ".json"
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		} else {
			return recoverySet{}, failure(ExitInvalid, "the dump has no recovery-set manifest", err)
		}
	}
	manifest, err := readRecoverySet(path)
	if err != nil {
		return recoverySet{}, failure(ExitInvalid, "read recovery manifest failed", err)
	}
	return manifest, nil
}

func readRecoverySet(path string) (recoverySet, error) {
	file, err := os.Open(path)
	if err != nil {
		return recoverySet{}, err
	}
	defer file.Close()
	var manifest recoverySet
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return recoverySet{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return recoverySet{}, fmt.Errorf("recovery manifest must contain one JSON document")
	}
	if manifest.SchemaVersion != recoverySchemaVersion || !recoverySetID.MatchString(manifest.ID) || manifest.CreatedAt.IsZero() || manifest.Environment == "" || manifest.ReleaseVersion == "" || len(manifest.Migrations) == 0 {
		return recoverySet{}, fmt.Errorf("unsupported or incomplete recovery manifest")
	}
	if err := validateRecoveryArtifact(manifest.Database, 1024); err != nil {
		return recoverySet{}, fmt.Errorf("invalid database artifact: %w", err)
	}
	if manifest.Uploads != nil {
		if err := validateRecoveryArtifact(*manifest.Uploads, 1); err != nil {
			return recoverySet{}, fmt.Errorf("invalid upload artifact: %w", err)
		}
	}
	if manifest.OffHost && manifest.ResticSnapshot == "" {
		return recoverySet{}, fmt.Errorf("off-host recovery manifest has no snapshot identity")
	}
	manifest.ManifestFile, _ = filepath.Abs(path)
	return manifest, nil
}

func validateRecoveryArtifact(artifact recoveryArtifact, minimumBytes int64) error {
	if artifact.File == "" || artifact.File != filepath.Base(artifact.File) || artifact.File == "." || artifact.File == ".." {
		return fmt.Errorf("file must be a base name")
	}
	if artifact.Bytes < minimumBytes {
		return fmt.Errorf("artifact is smaller than %d bytes", minimumBytes)
	}
	if len(artifact.SHA256) != sha256.Size*2 {
		return fmt.Errorf("SHA-256 digest must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(artifact.SHA256); err != nil {
		return fmt.Errorf("SHA-256 digest is invalid")
	}
	return nil
}

func (a *App) verifyBackup(ctx context.Context, config resolvedConfig, manifest recoverySet) (any, error) {
	directory := filepath.Dir(manifest.ManifestFile)
	databasePath := filepath.Join(directory, manifest.Database.File)
	if err := verifyArtifact(databasePath, manifest.Database); err != nil {
		return nil, failure(ExitVerification, "database checksum verification failed", err)
	}
	entries, err := a.verifyPostgresCatalog(ctx, config, databasePath)
	if err != nil {
		return nil, err
	}
	if manifest.Uploads != nil {
		if err := verifyArtifact(filepath.Join(directory, manifest.Uploads.File), *manifest.Uploads); err != nil {
			return nil, failure(ExitVerification, "upload checksum verification failed", err)
		}
	}
	return map[string]any{"recoverySet": manifest.ID, "verified": true, "catalogEntries": entries, "offHost": manifest.OffHost}, nil
}

func (a *App) verifyPostgresCatalog(ctx context.Context, config resolvedConfig, path string) (int, error) {
	var stdout, stderr bytes.Buffer
	var err error
	if config.PostgresDriver == "compose" {
		file, openErr := os.Open(path)
		if openErr != nil {
			return 0, failure(ExitVerification, "open PostgreSQL artifact failed", openErr)
		}
		err = a.Executor.Run(ctx, "docker", composeArguments(config, "exec", "-T", "postgres", "pg_restore", "--list"), nil, file, &stdout, &stderr)
		_ = file.Close()
	} else {
		if _, lookErr := a.LookPath("pg_restore"); lookErr != nil {
			return 0, failure(ExitPrecondition, "pg_restore is required", lookErr)
		}
		err = a.Executor.Run(ctx, "pg_restore", []string{"--list", path}, nil, nil, &stdout, &stderr)
	}
	if err != nil {
		return 0, failure(ExitVerification, "PostgreSQL catalog verification failed", sanitizedToolError(err, stderr.String()))
	}
	entries := 0
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ";") {
			entries++
		}
	}
	if entries == 0 {
		return 0, failure(ExitVerification, "PostgreSQL catalog is empty", nil)
	}
	return entries, nil
}

func (a *App) restoreBackup(ctx context.Context, config resolvedConfig, manifest recoverySet) (any, error) {
	verified, err := a.verifyBackup(ctx, config, manifest)
	if err != nil {
		return nil, err
	}
	dumpPath := filepath.Join(filepath.Dir(manifest.ManifestFile), manifest.Database.File)
	var diagnostics bytes.Buffer
	if err := a.restorePostgresDump(ctx, config, dumpPath, &diagnostics); err != nil {
		return nil, failure(ExitCommand, "PostgreSQL restore failed", sanitizedToolError(err, diagnostics.String()))
	}
	previousUploads := ""
	if manifest.Uploads != nil {
		archivePath := filepath.Join(filepath.Dir(manifest.ManifestFile), manifest.Uploads.File)
		staging := config.UploadsDir + ".restore-" + a.Now().UTC().Format("20060102T150405Z")
		if err := extractUploadsArchive(archivePath, staging); err != nil {
			return nil, failure(ExitCommand, "private upload restore failed", err)
		}
		if _, err := os.Stat(config.UploadsDir); err == nil {
			previousUploads = config.UploadsDir + ".pre-restore-" + a.Now().UTC().Format("20060102T150405Z")
			if err := os.Rename(config.UploadsDir, previousUploads); err != nil {
				return nil, failure(ExitCommand, "preserve current uploads before restore failed", err)
			}
		}
		if err := os.Rename(staging, config.UploadsDir); err != nil {
			return nil, failure(ExitCommand, "activate restored uploads failed", err)
		}
	}
	return map[string]any{"environment": config.Environment, "recoverySet": manifest.ID, "restored": true, "verification": verified, "previousUploads": previousUploads}, nil
}

func (a *App) createPostgresDump(ctx context.Context, config resolvedConfig, destination string, diagnostics io.Writer) error {
	if config.PostgresDriver == "compose" {
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		user := firstValue(config.lookup("POSTGRES_USER"), "hotelmate")
		databaseName := firstValue(config.lookup("POSTGRES_DB"), "hotelmate")
		args := composeArguments(config, "exec", "-T", "postgres", "pg_dump", "--username", user, "--dbname", databaseName, "--format=custom")
		runErr := a.Executor.Run(ctx, "docker", args, nil, nil, file, diagnostics)
		closeErr := file.Close()
		if runErr != nil {
			return runErr
		}
		return closeErr
	}
	postgresEnv, err := postgresEnvironment(config.DatabaseURL)
	if err != nil {
		return err
	}
	return a.Executor.Run(ctx, "pg_dump", []string{"--format=custom", "--file=" + destination}, postgresEnv, nil, ioDiscard{}, diagnostics)
}

func (a *App) restorePostgresDump(ctx context.Context, config resolvedConfig, source string, diagnostics io.Writer) error {
	common := []string{"--clean", "--if-exists", "--no-owner", "--no-privileges", "--exit-on-error"}
	if config.PostgresDriver == "compose" {
		file, err := os.Open(source)
		if err != nil {
			return err
		}
		defer file.Close()
		user := firstValue(config.lookup("POSTGRES_USER"), "hotelmate")
		databaseName := firstValue(config.lookup("POSTGRES_DB"), "hotelmate")
		args := composeArguments(config, append([]string{"exec", "-T", "postgres", "pg_restore"}, append(common, "--username", user, "--dbname", databaseName)...)...)
		return a.Executor.Run(ctx, "docker", args, nil, file, ioDiscard{}, diagnostics)
	}
	postgresEnv, err := postgresEnvironment(config.DatabaseURL)
	if err != nil {
		return err
	}
	return a.Executor.Run(ctx, "pg_restore", append(common, source), postgresEnv, nil, ioDiscard{}, diagnostics)
}

func postgresEnvironment(dsn string) ([]string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || parsed.User == nil {
		return nil, fmt.Errorf("expected a postgres:// URL")
	}
	databaseName := strings.TrimPrefix(parsed.EscapedPath(), "/")
	databaseName, err = url.PathUnescape(databaseName)
	if err != nil || databaseName == "" {
		return nil, fmt.Errorf("database name is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	password, _ := parsed.User.Password()
	environment := []string{
		"PGHOST=" + parsed.Hostname(), "PGPORT=" + port, "PGUSER=" + parsed.User.Username(),
		"PGPASSWORD=" + password, "PGDATABASE=" + databaseName, "PGCONNECT_TIMEOUT=10",
	}
	if sslmode := parsed.Query().Get("sslmode"); sslmode != "" {
		environment = append(environment, "PGSSLMODE="+sslmode)
	}
	return environment, nil
}

func inspectArtifact(path string) (recoveryArtifact, error) {
	file, err := os.Open(path)
	if err != nil {
		return recoveryArtifact{}, err
	}
	defer file.Close()
	hash := sha256.New()
	bytesWritten, err := io.Copy(hash, file)
	if err != nil {
		return recoveryArtifact{}, err
	}
	return recoveryArtifact{File: filepath.Base(path), SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: bytesWritten}, nil
}

func verifyArtifact(path string, expected recoveryArtifact) error {
	actual, err := inspectArtifact(path)
	if err != nil {
		return err
	}
	if actual.Bytes != expected.Bytes || actual.SHA256 != expected.SHA256 {
		return fmt.Errorf("artifact %s does not match manifest", expected.File)
	}
	return nil
}

func createUploadsArchive(root, destination string) (int, error) {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	failed := true
	defer func() {
		_ = file.Close()
		if failed {
			_ = os.Remove(destination)
		}
	}()
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	files := 0
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.Mode &= 0o700
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		files++
		return nil
	})
	if err == nil {
		err = tarWriter.Close()
	}
	if err == nil {
		err = gzipWriter.Close()
	}
	if err == nil {
		err = file.Close()
	}
	if err != nil {
		return 0, err
	}
	failed = false
	return files, nil
}

func extractUploadsArchive(archivePath, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	failed := true
	defer func() {
		if failed {
			_ = os.RemoveAll(destination)
		}
	}()
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			destinationFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(destinationFile, reader)
			closeErr := destinationFile.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
	failed = false
	return nil
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hotelmate-*.partial")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	failed := true
	defer func() {
		_ = temporary.Close()
		if failed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	failed = false
	return nil
}

func sanitizedToolError(err error, diagnostic string) error {
	message := strings.TrimSpace(diagnostic)
	if message == "" {
		return err
	}
	// Tool diagnostics can echo a URL. Redact common URL userinfo before the
	// message reaches stderr or a JSON envelope.
	fields := strings.Fields(message)
	for index, field := range fields {
		if strings.Contains(field, "://") && strings.Contains(field, "@") {
			if scheme, rest, ok := strings.Cut(field, "://"); ok {
				if _, host, ok := strings.Cut(rest, "@"); ok {
					fields[index] = scheme + "://[REDACTED]@" + host
				}
			}
		}
	}
	return fmt.Errorf("%s", strings.Join(fields, " "))
}
