package operations

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

const releaseSchemaVersion = "hotelmate.release/v1"

var immutableImageReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[a-f0-9]{64}$`)

type releaseManifest struct {
	SchemaVersion  string            `json:"schemaVersion"`
	ReleaseVersion string            `json:"releaseVersion"`
	Commit         string            `json:"commit"`
	CreatedAt      time.Time         `json:"createdAt"`
	APIImage       string            `json:"apiImage"`
	WebImage       string            `json:"webImage"`
	Migrations     []string          `json:"migrations,omitempty"`
	SBOMs          map[string]string `json:"sboms,omitempty"`
	Evidence       map[string]any    `json:"evidence,omitempty"`
	Source         string            `json:"-"`
}

type deploymentEvidence struct {
	SchemaVersion string           `json:"schemaVersion"`
	Environment   string           `json:"environment"`
	Status        string           `json:"status"`
	StartedAt     time.Time        `json:"startedAt"`
	FinishedAt    time.Time        `json:"finishedAt"`
	Release       releaseManifest  `json:"release"`
	Previous      *releaseManifest `json:"previousRelease,omitempty"`
	Backup        any              `json:"backup,omitempty"`
	Smoke         any              `json:"smoke,omitempty"`
	Error         string           `json:"error,omitempty"`
}

func (a *App) deploy(ctx context.Context, config resolvedConfig, path []string, options cliOptions) (any, error) {
	if len(path) != 2 {
		return nil, failure(ExitInvalid, "usage: hotelmate deploy preflight|apply|status|rollback", nil)
	}
	switch path[1] {
	case "preflight":
		release, err := readReleaseManifest(config.ReleaseFile)
		if err != nil {
			return nil, err
		}
		return a.deployPreflight(ctx, config, release)
	case "status":
		return a.deployStatus(ctx, config)
	case "apply":
		if !options.yes && !options.dryRun {
			return nil, failure(ExitPrecondition, fmt.Sprintf("deployment to %s requires --yes", config.Environment), nil)
		}
		release, err := readReleaseManifest(config.ReleaseFile)
		if err != nil {
			return nil, err
		}
		if options.dryRun {
			preflight, err := a.deployPreflight(ctx, config, release)
			return map[string]any{"dryRun": true, "release": release, "preflight": preflight}, err
		}
		return a.applyRelease(ctx, config, release)
	case "rollback":
		if !options.yes && !options.dryRun {
			return nil, failure(ExitPrecondition, fmt.Sprintf("rollback of %s requires --yes", config.Environment), nil)
		}
		release, err := readReleaseManifest(config.ReleaseFile)
		if err != nil {
			return nil, err
		}
		if options.dryRun {
			return map[string]any{"dryRun": true, "rollbackTo": release}, nil
		}
		return a.rollbackRelease(ctx, config, release)
	default:
		return nil, failure(ExitInvalid, "usage: hotelmate deploy preflight|apply|status|rollback", nil)
	}
}

func readReleaseManifest(path string) (releaseManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return releaseManifest{}, failure(ExitPrecondition, "release manifest is unavailable", err)
	}
	defer file.Close()
	var release releaseManifest
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&release); err != nil {
		return releaseManifest{}, failure(ExitInvalid, "release manifest is invalid", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return releaseManifest{}, failure(ExitInvalid, "release manifest must contain one JSON document", err)
	}
	release.Source, _ = filepath.Abs(path)
	if release.SchemaVersion != releaseSchemaVersion || release.ReleaseVersion == "" || release.Commit == "" || release.CreatedAt.IsZero() || release.APIImage == "" || release.WebImage == "" || len(release.Migrations) == 0 || release.SBOMs["api"] == "" || release.SBOMs["web"] == "" || len(release.Evidence) == 0 {
		return releaseManifest{}, failure(ExitInvalid, "release manifest is incomplete or has an unsupported schema", nil)
	}
	return release, nil
}

func (a *App) deployPreflight(ctx context.Context, config resolvedConfig, release releaseManifest) (any, error) {
	checks := []checkResult{}
	add := func(name string, err error, success string) {
		if err == nil {
			checks = append(checks, checkResult{Name: name, OK: true, Message: success})
		} else {
			checks = append(checks, checkResult{Name: name, Message: err.Error()})
		}
	}
	_, validationErr := a.validateConfig(config)
	add("configuration", validationErr, "valid")
	_, dockerErr := a.LookPath("docker")
	add("docker", dockerErr, "available")
	_, composeErr := os.Stat(config.ComposeFile)
	add("compose-file", composeErr, "available")
	if config.Environment == "staging" || config.Environment == "production" {
		if !immutableImageReference.MatchString(release.APIImage) || !immutableImageReference.MatchString(release.WebImage) {
			add("immutable-images", fmt.Errorf("staging/production images must be digest references"), "")
		} else {
			add("immutable-images", nil, "digest pinned")
		}
		if !slices.Equal(release.Migrations, database.MigrationVersions()) {
			add("migration-set", fmt.Errorf("release migration set does not match the operations binary"), "")
		} else {
			add("migration-set", nil, "matches operations binary")
		}
		if release.SBOMs["api"] == "" || release.SBOMs["web"] == "" || len(release.Evidence) == 0 {
			add("release-evidence", fmt.Errorf("SBOM and CI evidence are required"), "")
		} else {
			add("release-evidence", nil, "SBOM and CI evidence recorded")
		}
		info, err := os.Stat(config.EnvFile)
		if err == nil && info.Mode().Perm()&0o077 != 0 {
			err = fmt.Errorf("%s must use mode 600", config.EnvFile)
		}
		add("environment-file", err, "protected")
		if config.CosignIdentity == "" || config.CosignIssuer == "" {
			add("signature-policy", fmt.Errorf("COSIGN_CERTIFICATE_IDENTITY and COSIGN_CERTIFICATE_OIDC_ISSUER are required"), "")
		} else if _, err := a.LookPath("cosign"); err != nil {
			add("signature-policy", err, "")
		} else {
			add("signature-policy", nil, "configured")
		}
	}
	if config.Environment == "production" {
		add("off-host-backup", productionBackupPolicy(config), productionBackupPolicySuccess(config))
	}
	var composeDiagnostic bytes.Buffer
	env := releaseEnvironment(release)
	err := a.Executor.Run(ctx, "docker", composeArguments(config, "config", "--quiet"), env, nil, ioDiscard{}, &composeDiagnostic)
	if err != nil {
		err = sanitizedToolError(err, composeDiagnostic.String())
	}
	add("compose-config", err, "valid")
	for _, image := range []struct{ name, reference string }{{"api-image", release.APIImage}, {"web-image", release.WebImage}} {
		var diagnostic bytes.Buffer
		inspectArgs := []string{"buildx", "imagetools", "inspect", image.reference}
		if config.Environment == "development" && !strings.Contains(image.reference, "@sha256:") {
			inspectArgs = []string{"image", "inspect", image.reference}
		}
		err := a.Executor.Run(ctx, "docker", inspectArgs, nil, nil, ioDiscard{}, &diagnostic)
		if err != nil {
			err = sanitizedToolError(err, diagnostic.String())
		}
		add(image.name, err, "registry access verified")
		if err == nil && (config.Environment == "staging" || config.Environment == "production") && config.CosignIdentity != "" {
			verifyArgs := []string{"verify", "--certificate-identity", config.CosignIdentity, "--certificate-oidc-issuer", config.CosignIssuer, image.reference}
			diagnostic.Reset()
			signatureErr := a.Executor.Run(ctx, "cosign", verifyArgs, nil, nil, ioDiscard{}, &diagnostic)
			if signatureErr == nil {
				attestationArgs := []string{"verify-attestation", "--type", "spdxjson", "--certificate-identity", config.CosignIdentity, "--certificate-oidc-issuer", config.CosignIssuer, image.reference}
				signatureErr = a.Executor.Run(ctx, "cosign", attestationArgs, nil, nil, ioDiscard{}, &diagnostic)
			}
			if signatureErr != nil {
				signatureErr = sanitizedToolError(signatureErr, diagnostic.String())
			}
			add(image.name+"-signature", signatureErr, "signature and SPDX attestation verified")
		}
	}
	failed := 0
	for _, check := range checks {
		if !check.OK {
			failed++
		}
	}
	data := map[string]any{"environment": config.Environment, "release": release, "checks": checks, "failed": failed}
	if failed > 0 {
		return data, failure(ExitPrecondition, fmt.Sprintf("deployment preflight found %d failed checks", failed), nil)
	}
	return data, nil
}

func productionBackupPolicy(config resolvedConfig) error {
	if config.ResticRepository != "" && config.ResticPassword != "" {
		return nil
	}
	if config.OffHostDeferred {
		return nil
	}
	return fmt.Errorf("RESTIC_REPOSITORY and RESTIC_PASSWORD are required unless the owner-approved deferral is recorded")
}

func productionBackupPolicySuccess(config resolvedConfig) string {
	if config.ResticRepository != "" && config.ResticPassword != "" {
		return "configured"
	}
	return "owner-approved deferral recorded; local recovery checkpoint remains mandatory"
}

func (a *App) applyRelease(ctx context.Context, config resolvedConfig, release releaseManifest) (any, error) {
	releaseLock, err := acquireEnvironmentLock(config.EvidenceDir, config.Environment)
	if err != nil {
		return nil, err
	}
	defer releaseLock()
	started := a.Now().UTC()
	preflight, err := a.deployPreflight(ctx, config, release)
	if err != nil {
		return preflight, err
	}
	previous, _ := currentRelease(config.EvidenceDir, config.Environment)
	evidence := deploymentEvidence{SchemaVersion: SchemaVersion, Environment: config.Environment, Status: "started", StartedAt: started, Release: release, Previous: previous}
	if config.Environment == "production" {
		backup, backupErr := a.createBackup(ctx, config)
		evidence.Backup = backup
		if backupErr != nil {
			evidence.Status = "failed"
			evidence.Error = backupErr.Error()
			evidence.FinishedAt = a.Now().UTC()
			_ = writeDeploymentEvidence(config.EvidenceDir, evidence)
			return evidence, backupErr
		}
	}
	if err := a.activateRelease(ctx, config, release, true); err != nil {
		evidence.Status = "failed"
		evidence.Error = err.Error()
		if previous != nil {
			if rollbackErr := a.activateRelease(ctx, config, *previous, false); rollbackErr == nil {
				evidence.Status = "failed_rolled_back"
			} else {
				evidence.Error += "; automatic rollback failed: " + rollbackErr.Error()
			}
		}
		evidence.FinishedAt = a.Now().UTC()
		_ = writeDeploymentEvidence(config.EvidenceDir, evidence)
		return evidence, err
	}
	smoke, smokeErr := a.waitForSmoke(ctx, config, 60*time.Second)
	evidence.Smoke = smoke
	if smokeErr != nil {
		evidence.Status = "failed"
		evidence.Error = smokeErr.Error()
		if previous != nil {
			if rollbackErr := a.activateRelease(ctx, config, *previous, false); rollbackErr == nil {
				evidence.Status = "failed_rolled_back"
			} else {
				evidence.Error += "; automatic rollback failed: " + rollbackErr.Error()
			}
		}
		evidence.FinishedAt = a.Now().UTC()
		_ = writeDeploymentEvidence(config.EvidenceDir, evidence)
		return evidence, smokeErr
	}
	evidence.Status = "succeeded"
	evidence.FinishedAt = a.Now().UTC()
	if err := writeDeploymentEvidence(config.EvidenceDir, evidence); err != nil {
		return evidence, failure(ExitVerification, "record deployment evidence failed", err)
	}
	if err := writeJSONAtomic(currentReleasePath(config.EvidenceDir, config.Environment), release, 0o600); err != nil {
		return evidence, failure(ExitVerification, "record current release failed", err)
	}
	return evidence, nil
}

func (a *App) rollbackRelease(ctx context.Context, config resolvedConfig, release releaseManifest) (any, error) {
	releaseLock, err := acquireEnvironmentLock(config.EvidenceDir, config.Environment)
	if err != nil {
		return nil, err
	}
	defer releaseLock()
	previous, _ := currentRelease(config.EvidenceDir, config.Environment)
	evidence := deploymentEvidence{SchemaVersion: SchemaVersion, Environment: config.Environment, Status: "rollback_started", StartedAt: a.Now().UTC(), Release: release, Previous: previous}
	if err := a.activateRelease(ctx, config, release, false); err != nil {
		evidence.Status = "rollback_failed"
		evidence.Error = err.Error()
		evidence.FinishedAt = a.Now().UTC()
		_ = writeDeploymentEvidence(config.EvidenceDir, evidence)
		return evidence, err
	}
	smoke, err := a.waitForSmoke(ctx, config, 60*time.Second)
	evidence.Smoke = smoke
	if err != nil {
		evidence.Status = "rollback_verification_failed"
		evidence.Error = err.Error()
		evidence.FinishedAt = a.Now().UTC()
		_ = writeDeploymentEvidence(config.EvidenceDir, evidence)
		return evidence, err
	}
	evidence.Status = "rolled_back"
	evidence.FinishedAt = a.Now().UTC()
	if err := writeDeploymentEvidence(config.EvidenceDir, evidence); err != nil {
		return evidence, failure(ExitVerification, "record rollback evidence failed", err)
	}
	if err := writeJSONAtomic(currentReleasePath(config.EvidenceDir, config.Environment), release, 0o600); err != nil {
		return evidence, failure(ExitVerification, "record current release failed", err)
	}
	return evidence, nil
}

func (a *App) activateRelease(ctx context.Context, config resolvedConfig, release releaseManifest, runMigrations bool) error {
	releaseEnvPath := managedReleaseEnvPath(config.EvidenceDir, config.Environment)
	if err := writeManagedReleaseEnv(releaseEnvPath, release); err != nil {
		return failure(ExitCommand, "write managed release environment failed", err)
	}
	deploymentConfig := config
	var diagnostics bytes.Buffer
	env := releaseEnvironment(release)
	if config.Environment != "development" || strings.Contains(release.APIImage, "@sha256:") || strings.Contains(release.WebImage, "@sha256:") {
		if err := a.Executor.Run(ctx, "docker", composeArguments(deploymentConfig, "pull", "api", "web"), env, nil, ioDiscard{}, &diagnostics); err != nil {
			return failure(ExitCommand, "image pull failed", sanitizedToolError(err, diagnostics.String()))
		}
	}
	diagnostics.Reset()
	if err := a.Executor.Run(ctx, "docker", composeArguments(deploymentConfig, "up", "-d", "postgres"), env, nil, ioDiscard{}, &diagnostics); err != nil {
		return failure(ExitCommand, "database service start failed", sanitizedToolError(err, diagnostics.String()))
	}
	if runMigrations {
		diagnostics.Reset()
		args := composeArguments(deploymentConfig, "run", "--rm", "--entrypoint", "/app/hotelmate", "api", "--environment", config.Environment, "migrate", "up", "--yes")
		if err := a.Executor.Run(ctx, "docker", args, env, nil, ioDiscard{}, &diagnostics); err != nil {
			return failure(ExitCommand, "release migration failed", sanitizedToolError(err, diagnostics.String()))
		}
	}
	diagnostics.Reset()
	if err := a.Executor.Run(ctx, "docker", composeArguments(deploymentConfig, "up", "-d", "--remove-orphans"), env, nil, ioDiscard{}, &diagnostics); err != nil {
		return failure(ExitCommand, "release activation failed", sanitizedToolError(err, diagnostics.String()))
	}
	return nil
}

func (a *App) deployStatus(ctx context.Context, config resolvedConfig) (any, error) {
	var stdout, stderr bytes.Buffer
	if err := a.Executor.Run(ctx, "docker", composeArguments(config, "ps", "--format", "json"), nil, nil, &stdout, &stderr); err != nil {
		return nil, failure(ExitCommand, "deployment status failed", sanitizedToolError(err, stderr.String()))
	}
	release, _ := currentRelease(config.EvidenceDir, config.Environment)
	return map[string]any{"environment": config.Environment, "release": release, "compose": strings.TrimSpace(stdout.String())}, nil
}

func composeArguments(config resolvedConfig, command ...string) []string {
	args := []string{"compose"}
	if _, err := os.Stat(config.EnvFile); err == nil {
		args = append(args, "--env-file", config.EnvFile)
	}
	managed := managedReleaseEnvPath(config.EvidenceDir, config.Environment)
	if _, err := os.Stat(managed); err == nil {
		args = append(args, "--env-file", managed)
	}
	args = append(args, "-f", config.ComposeFile)
	if observability := managedObservabilityCompose(config); observability != "" {
		args = append(args, "-f", observability)
	}
	return append(args, command...)
}

func managedObservabilityCompose(config resolvedConfig) string {
	if config.Environment == "development" {
		return ""
	}
	path := filepath.Join(filepath.Dir(config.ComposeFile), "docker-compose.observability.yml")
	if filepath.Clean(path) == filepath.Clean(config.ComposeFile) {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func releaseEnvironment(release releaseManifest) []string {
	return []string{"HOTELMATE_API_IMAGE=" + release.APIImage, "HOTELMATE_WEB_IMAGE=" + release.WebImage, "API_VERSION=" + release.ReleaseVersion, "RELEASE_COMMIT=" + release.Commit}
}

func writeManagedReleaseEnv(path string, release releaseManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content := "HOTELMATE_API_IMAGE=" + release.APIImage + "\nHOTELMATE_WEB_IMAGE=" + release.WebImage + "\nAPI_VERSION=" + release.ReleaseVersion + "\nRELEASE_COMMIT=" + release.Commit + "\n"
	temporary, err := os.CreateTemp(filepath.Dir(path), ".release-*.partial")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func acquireEnvironmentLock(directory, environment string) (func(), error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, failure(ExitPrecondition, "create deployment lock directory failed", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, failure(ExitPrecondition, "protect deployment lock directory failed", err)
	}
	path := filepath.Join(directory, "."+environment+".lock")
	file, err := createEnvironmentLock(path)
	if err != nil {
		if !os.IsExist(err) {
			return nil, failure(ExitPrecondition, "create environment lock failed", err)
		}
		stale, staleErr := archiveStaleEnvironmentLock(path)
		if staleErr != nil {
			return nil, failure(ExitPrecondition, "another deploy, migration, restore, or recovery drill holds the environment lock", staleErr)
		}
		if !stale {
			return nil, failure(ExitPrecondition, "another deploy, migration, restore, or recovery drill holds the environment lock", err)
		}
		file, err = createEnvironmentLock(path)
		if err != nil {
			return nil, failure(ExitPrecondition, "acquire environment lock after archiving a stale owner failed", err)
		}
	}
	var owner environmentLock
	if _, err := file.Seek(0, 0); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, failure(ExitPrecondition, "read environment lock ownership failed", err)
	}
	if err := json.NewDecoder(file).Decode(&owner); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, failure(ExitPrecondition, "read environment lock ownership failed", err)
	}
	_ = file.Close()
	return func() {
		current, err := readEnvironmentLock(path)
		if err == nil && current.Token == owner.Token {
			_ = os.Remove(path)
		}
	}, nil
}

type environmentLock struct {
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	CreatedAt time.Time `json:"createdAt"`
	Token     string    `json:"token"`
}

func createEnvironmentLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	hostname, _ := os.Hostname()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	owner := environmentLock{PID: os.Getpid(), Hostname: hostname, CreatedAt: time.Now().UTC(), Token: hex.EncodeToString(random)}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(owner); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	failed = false
	return file, nil
}

func readEnvironmentLock(path string) (environmentLock, error) {
	file, err := os.Open(path)
	if err != nil {
		return environmentLock{}, err
	}
	defer file.Close()
	var owner environmentLock
	if err := json.NewDecoder(io.LimitReader(file, 4096)).Decode(&owner); err != nil {
		return environmentLock{}, err
	}
	if owner.PID <= 0 || owner.Hostname == "" || owner.Token == "" || owner.CreatedAt.IsZero() {
		return environmentLock{}, fmt.Errorf("lock ownership metadata is incomplete")
	}
	return owner, nil
}

func archiveStaleEnvironmentLock(path string) (bool, error) {
	owner, err := readEnvironmentLock(path)
	if err != nil {
		return false, fmt.Errorf("existing lock cannot be verified safely: %w", err)
	}
	hostname, _ := os.Hostname()
	if owner.Hostname != hostname {
		return false, fmt.Errorf("lock belongs to host %s and requires operator review", owner.Hostname)
	}
	if err := syscall.Kill(owner.PID, 0); err == nil || err == syscall.EPERM {
		return false, nil
	} else if err != syscall.ESRCH {
		return false, fmt.Errorf("check lock owner process %d: %w", owner.PID, err)
	}
	archive := path + ".stale." + time.Now().UTC().Format("20060102T150405.000000000Z")
	if err := os.Rename(path, archive); err != nil {
		return false, err
	}
	return true, nil
}

func writeDeploymentEvidence(directory string, evidence deploymentEvidence) error {
	name := evidence.StartedAt.UTC().Format("20060102T150405.000000000Z") + "-" + evidence.Status + ".json"
	return writeJSONAtomic(filepath.Join(directory, evidence.Environment, name), evidence, 0o600)
}

func managedReleaseEnvPath(directory, environment string) string {
	return filepath.Join(directory, environment, "current-release.env")
}

func currentReleasePath(directory, environment string) string {
	return filepath.Join(directory, environment, "current-release.json")
}

func currentRelease(directory, environment string) (*releaseManifest, error) {
	release, err := readReleaseManifest(currentReleasePath(directory, environment))
	if err != nil {
		return nil, err
	}
	return &release, nil
}
