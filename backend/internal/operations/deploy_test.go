package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadReleaseManifestRequiresCompleteSingleDocument(t *testing.T) {
	directory := t.TempDir()
	valid := releaseManifest{
		SchemaVersion:  releaseSchemaVersion,
		ReleaseVersion: "0.7.0-test",
		Commit:         strings.Repeat("a", 40),
		CreatedAt:      time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC),
		APIImage:       "hotelmate-api:test",
		WebImage:       "hotelmate-web:test",
		Migrations:     []string{"migration"},
		SBOMs:          map[string]string{"api": "api.spdx.json", "web": "web.spdx.json"},
		Evidence:       map[string]any{"ciRun": "123"},
	}
	path := filepath.Join(directory, "release.json")
	if err := writeJSONAtomic(path, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseManifest(path); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{}\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseManifest(path); err == nil || !strings.Contains(err.Error(), "one JSON document") {
		t.Fatalf("trailing document error = %v", err)
	}
}

func TestImmutableImageReferenceRequiresFullSHA256Digest(t *testing.T) {
	valid := "ghcr.io/tablazorg/hotelmate/api@sha256:" + strings.Repeat("a", 64)
	if !immutableImageReference.MatchString(valid) {
		t.Fatalf("valid digest reference rejected: %s", valid)
	}
	for _, invalid := range []string{
		"ghcr.io/tablazorg/hotelmate/api:latest",
		"ghcr.io/tablazorg/hotelmate/api@sha256:abc",
		"ghcr.io/tablazorg/hotelmate/api@sha256:" + strings.Repeat("G", 64),
		"ghcr.io/tablazorg/hotelmate/api@sha256:" + strings.Repeat("a", 64) + " extra",
	} {
		if immutableImageReference.MatchString(invalid) {
			t.Fatalf("invalid digest reference accepted: %s", invalid)
		}
	}
}

func TestProductionBackupPolicyRequiresConfigurationOrExplicitDeferral(t *testing.T) {
	if err := productionBackupPolicy(resolvedConfig{}); err == nil || !strings.Contains(err.Error(), "owner-approved deferral") {
		t.Fatalf("missing production backup policy error = %v", err)
	}
	if err := productionBackupPolicy(resolvedConfig{OffHostDeferred: true}); err != nil {
		t.Fatalf("explicit owner deferral rejected: %v", err)
	}
	configured := resolvedConfig{ResticRepository: "s3:example/bucket", ResticPassword: "secret"}
	if err := productionBackupPolicy(configured); err != nil {
		t.Fatalf("configured off-host backup rejected: %v", err)
	}
}

func TestComposeArgumentsKeepManagedObservabilityInTheDeploymentProject(t *testing.T) {
	directory := t.TempDir()
	production := filepath.Join(directory, "docker-compose.production.yml")
	observability := filepath.Join(directory, "docker-compose.observability.yml")
	for _, path := range []string{production, observability} {
		if err := os.WriteFile(path, []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	managed := strings.Join(composeArguments(resolvedConfig{
		Environment: "production",
		ComposeFile: production,
		EvidenceDir: filepath.Join(directory, "evidence"),
	}, "up", "-d", "--remove-orphans"), " ")
	if !strings.Contains(managed, "-f "+production+" -f "+observability) {
		t.Fatalf("managed compose arguments do not include observability: %s", managed)
	}

	development := strings.Join(composeArguments(resolvedConfig{
		Environment: "development",
		ComposeFile: production,
		EvidenceDir: filepath.Join(directory, "evidence"),
	}, "up", "-d"), " ")
	if strings.Contains(development, observability) {
		t.Fatalf("development unexpectedly includes managed observability: %s", development)
	}
}

func TestEnvironmentLockRejectsLiveOwnerAndReleasesByToken(t *testing.T) {
	directory := t.TempDir()
	release, err := acquireEnvironmentLock(directory, "staging")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireEnvironmentLock(directory, "staging"); err == nil || !strings.Contains(err.Error(), "holds the environment lock") {
		t.Fatalf("concurrent lock error = %v", err)
	}
	release()
	releaseAgain, err := acquireEnvironmentLock(directory, "staging")
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	releaseAgain()
}

func TestEnvironmentLockArchivesVerifiedStaleOwner(t *testing.T) {
	directory := t.TempDir()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ".production.lock")
	stale := environmentLock{
		PID:       99999999,
		Hostname:  hostname,
		CreatedAt: time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC),
		Token:     strings.Repeat("a", 32),
	}
	if err := writeJSONAtomic(path, stale, 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireEnvironmentLock(directory, "production")
	if err != nil {
		t.Fatalf("verified stale lock was not recovered: %v", err)
	}
	release()
	archives, err := filepath.Glob(path + ".stale.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 1 {
		t.Fatalf("stale lock archives = %v", archives)
	}
}
