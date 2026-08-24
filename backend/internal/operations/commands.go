package operations

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
)

func (a *App) migrate(ctx context.Context, config resolvedConfig, path []string, options cliOptions) (any, error) {
	if len(path) != 2 || (path[1] != "status" && path[1] != "up") {
		return nil, failure(ExitInvalid, "usage: hotelmate migrate status|up [--dry-run]", nil)
	}
	if path[1] == "up" && !options.dryRun && !options.yes {
		return nil, failure(ExitPrecondition, fmt.Sprintf("migration of %s requires --yes", config.Environment), nil)
	}
	databaseContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	db, err := database.Open(databaseContext, config.DatabaseURL)
	if err != nil {
		return nil, failure(ExitPrecondition, "database connection failed", err)
	}
	defer database.Close(db)
	statuses, err := database.MigrationStatuses(db)
	if err != nil {
		return nil, failure(ExitCommand, "migration status failed", err)
	}
	pending := 0
	for _, status := range statuses {
		if !status.Applied {
			pending++
		}
	}
	if path[1] == "status" || options.dryRun {
		return map[string]any{"environment": config.Environment, "pending": pending, "migrations": statuses, "dryRun": options.dryRun}, nil
	}
	releaseLock, err := acquireEnvironmentLock(config.EvidenceDir, config.Environment)
	if err != nil {
		return nil, err
	}
	defer releaseLock()
	if err := database.Migrate(db); err != nil {
		return nil, failure(ExitCommand, "database migration failed", err)
	}
	statuses, err = database.MigrationStatuses(db)
	if err != nil {
		return nil, failure(ExitVerification, "migration verification failed", err)
	}
	return map[string]any{"environment": config.Environment, "applied": pending, "pending": 0, "migrations": statuses}, nil
}

type checkResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func (a *App) doctor(ctx context.Context, config resolvedConfig) (any, error) {
	checks := []checkResult{}
	add := func(name string, err error, success string) {
		if err != nil {
			checks = append(checks, checkResult{Name: name, Message: err.Error()})
			return
		}
		checks = append(checks, checkResult{Name: name, OK: true, Message: success})
	}
	_, configErr := a.validateConfig(config)
	add("configuration", configErr, "valid")
	tools := []string{"docker", "git"}
	if config.PostgresDriver == "direct" {
		tools = append(tools, "pg_dump", "pg_restore")
	}
	for _, tool := range tools {
		_, err := a.LookPath(tool)
		add("tool:"+tool, err, "available")
	}
	var dockerInfo bytes.Buffer
	dockerInfoErr := a.Executor.Run(ctx, "docker", []string{"info", "--format", "{{.ServerVersion}}"}, nil, nil, &dockerInfo, ioDiscard{})
	add("docker-daemon", dockerInfoErr, "reachable")
	if config.ResticRepository != "" {
		_, err := a.LookPath("restic")
		add("tool:restic", err, "available")
		if err == nil {
			var diagnostic bytes.Buffer
			resticEnv := []string{"RESTIC_REPOSITORY=" + config.ResticRepository, "RESTIC_PASSWORD=" + config.ResticPassword}
			err = a.Executor.Run(ctx, "restic", []string{"snapshots", "--json", "--latest", "1"}, resticEnv, nil, ioDiscard{}, &diagnostic)
			if err != nil {
				err = sanitizedToolError(err, diagnostic.String())
			}
			add("backup-destination", err, "repository accessible")
		}
	}
	if config.Environment == "staging" || config.Environment == "production" {
		_, err := a.LookPath("cosign")
		add("tool:cosign", err, "available")
	}
	stat := syscall.Statfs_t{}
	diskPath := config.BackupDir
	var err error
	for {
		err = syscall.Statfs(diskPath, &stat)
		if err == nil || !os.IsNotExist(err) {
			break
		}
		parent := filepath.Dir(diskPath)
		if parent == diskPath {
			break
		}
		diskPath = parent
	}
	if err == nil {
		available := uint64(stat.Bavail) * uint64(stat.Bsize)
		if available < config.MinDiskBytes {
			err = fmt.Errorf("only %d bytes available; require %d", available, config.MinDiskBytes)
		}
	}
	add("disk", err, "sufficient backup headroom")

	databaseContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	db, err := database.Open(databaseContext, config.DatabaseURL)
	cancel()
	if err == nil && db != nil && config.PostgresDriver == "direct" {
		var serverVersionNumber int
		versionErr := db.Raw("SHOW server_version_num").Scan(&serverVersionNumber).Error
		var versionOutput bytes.Buffer
		if versionErr == nil {
			versionErr = a.Executor.Run(ctx, "pg_dump", []string{"--version"}, nil, nil, &versionOutput, ioDiscard{})
		}
		var clientMajor int
		if versionErr == nil {
			if _, scanErr := fmt.Sscanf(versionOutput.String(), "pg_dump (PostgreSQL) %d", &clientMajor); scanErr != nil {
				versionErr = scanErr
			} else if clientMajor != serverVersionNumber/10000 {
				versionErr = fmt.Errorf("pg_dump major %d does not match PostgreSQL major %d", clientMajor, serverVersionNumber/10000)
			}
		}
		add("postgres-client-version", versionErr, "matches server")
	}
	if db != nil {
		_ = database.Close(db)
	}
	add("database", err, "ready")

	parsed, parseErr := url.Parse(config.BaseURL)
	add("target-url", parseErr, "valid")
	if parseErr == nil && parsed.Hostname() != "" {
		_, err := net.DefaultResolver.LookupHost(ctx, parsed.Hostname())
		add("dns", err, "resolved")
		if parsed.Scheme == "https" {
			port := parsed.Port()
			if port == "" {
				port = "443"
			}
			dialer := &net.Dialer{Timeout: 10 * time.Second}
			connection, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(parsed.Hostname(), port), &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12})
			if connection != nil {
				_ = connection.Close()
			}
			add("tls", err, "valid TLS 1.2+ connection")
		}
		healthURL := strings.TrimRight(config.BaseURL, "/") + "/healthz"
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if requestErr == nil {
			client := &http.Client{Timeout: 10 * time.Second}
			response, responseErr := client.Do(request)
			requestErr = responseErr
			if response != nil {
				_ = response.Body.Close()
				if requestErr == nil && (response.StatusCode < 200 || response.StatusCode >= 300) {
					requestErr = fmt.Errorf("health endpoint returned %s", response.Status)
				}
			}
		}
		add("target-connectivity", requestErr, "health endpoint reachable")
	}
	composeArgs := composeArguments(config, "config", "--quiet")
	var diagnostic bytes.Buffer
	err = a.Executor.Run(ctx, "docker", composeArgs, nil, nil, ioDiscard{}, &diagnostic)
	if err != nil && diagnostic.Len() > 0 {
		err = sanitizedToolError(err, diagnostic.String())
	}
	add("compose", err, "configuration valid")

	if release, releaseErr := readReleaseManifest(config.ReleaseFile); releaseErr == nil {
		add("release-manifest", nil, "valid")
		for _, image := range []struct{ name, reference string }{{"api", release.APIImage}, {"web", release.WebImage}} {
			var registryDiagnostic bytes.Buffer
			inspectArgs := []string{"buildx", "imagetools", "inspect", image.reference}
			if config.Environment == "development" && !strings.Contains(image.reference, "@sha256:") {
				inspectArgs = []string{"image", "inspect", image.reference}
			}
			registryErr := a.Executor.Run(ctx, "docker", inspectArgs, nil, nil, ioDiscard{}, &registryDiagnostic)
			if registryErr != nil {
				registryErr = sanitizedToolError(registryErr, registryDiagnostic.String())
			}
			add("registry:"+image.name, registryErr, "image accessible")
		}
	} else if config.Environment == "staging" || config.Environment == "production" {
		add("release-manifest", releaseErr, "")
	} else {
		add("registry", nil, "release image check deferred until a manifest is supplied")
	}

	failed := 0
	for _, check := range checks {
		if !check.OK {
			failed++
		}
	}
	data := map[string]any{"environment": config.Environment, "platform": runtime.GOOS + "/" + runtime.GOARCH, "checks": checks, "failed": failed}
	if failed > 0 {
		return data, failure(ExitPrecondition, fmt.Sprintf("doctor found %d failed checks", failed), nil)
	}
	return data, nil
}

type ioDiscard struct{}

func (ioDiscard) Write(body []byte) (int, error) { return len(body), nil }

func (a *App) retention(ctx context.Context, config resolvedConfig, path []string, options cliOptions) (any, error) {
	if len(path) != 2 || (path[1] != "purge-documents" && path[1] != "purge-messages") {
		return nil, failure(ExitInvalid, "usage: hotelmate retention purge-documents|purge-messages", nil)
	}
	if !options.yes {
		return nil, failure(ExitPrecondition, fmt.Sprintf("retention purge in %s requires --yes", config.Environment), nil)
	}
	databaseContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	db, err := database.Open(databaseContext, config.DatabaseURL)
	if err != nil {
		return nil, failure(ExitPrecondition, "database connection failed", err)
	}
	defer database.Close(db)
	repository := store.New(db)
	if path[1] == "purge-messages" {
		purged, err := repository.PurgeExpiredMessages(databaseContext, a.Now().UTC())
		if err != nil {
			return nil, failure(ExitCommand, "message purge failed", err)
		}
		return map[string]any{"environment": config.Environment, "purged": purged, "resource": "messages"}, nil
	}
	storage, err := documents.NewLocalStorage(config.UploadsDir, config.Application.DocumentMaxBytes)
	if err != nil {
		return nil, failure(ExitInvalid, "document storage is invalid", err)
	}
	purged, failed := 0, 0
	for {
		items, err := repository.ListExpiredDocuments(databaseContext, a.Now().UTC(), 100)
		if err != nil {
			return nil, failure(ExitCommand, "list expired documents failed", err)
		}
		if len(items) == 0 {
			break
		}
		batchPurged := 0
		for _, item := range items {
			if err := storage.Delete(databaseContext, item.DocumentStorageKey); err != nil {
				failed++
				continue
			}
			if err := repository.MarkDocumentDeleted(databaseContext, item.ID, a.Now().UTC()); err != nil {
				failed++
				continue
			}
			purged++
			batchPurged++
		}
		if len(items) < 100 || batchPurged == 0 {
			break
		}
	}
	for {
		documentItems, signatureItems, err := repository.ListExpiredArrivalEvidence(databaseContext, a.Now().UTC(), 100)
		if err != nil {
			return nil, failure(ExitCommand, "list expired arrival evidence failed", err)
		}
		if len(documentItems) == 0 && len(signatureItems) == 0 {
			break
		}
		batchPurged := 0
		for _, item := range documentItems {
			if err := storage.Delete(databaseContext, item.StorageKey); err != nil {
				failed++
				continue
			}
			if err := repository.MarkArrivalDocumentDeleted(databaseContext, item.ID, a.Now().UTC()); err != nil {
				failed++
				continue
			}
			purged++
			batchPurged++
		}
		for _, item := range signatureItems {
			if err := storage.Delete(databaseContext, item.SignatureStorageKey); err != nil {
				failed++
				continue
			}
			if err := repository.MarkArrivalSignatureDeleted(databaseContext, item.ID, a.Now().UTC()); err != nil {
				failed++
				continue
			}
			purged++
			batchPurged++
		}
		if (len(documentItems) < 100 && len(signatureItems) < 100) || batchPurged == 0 {
			break
		}
	}
	data := map[string]any{"environment": config.Environment, "purged": purged, "failed": failed, "resource": "documents"}
	if failed > 0 {
		return data, failure(ExitVerification, fmt.Sprintf("%d documents could not be purged", failed), nil)
	}
	return data, nil
}
