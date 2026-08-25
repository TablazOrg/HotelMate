package operations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

func TestRecoverySetAtSelectsNewestSetBeforeRequestedPoint(t *testing.T) {
	directory := t.TempDir()
	base := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	for index, createdAt := range []time.Time{base, base.Add(2 * time.Hour)} {
		manifest := recoverySet{
			SchemaVersion:  recoverySchemaVersion,
			ID:             "hotelmate-" + createdAt.Format("20060102T150405Z"),
			CreatedAt:      createdAt,
			Environment:    "production",
			ReleaseVersion: "0.7.0-test",
			Database:       recoveryArtifact{File: "database.dump", SHA256: strings.Repeat("a", 64), Bytes: 2048},
			Migrations:     []database.MigrationStatus{{Version: "test", Applied: true}},
		}
		path := filepath.Join(directory, manifest.ID+".json")
		if err := writeJSONAtomic(path, manifest, 0o600); err != nil {
			t.Fatalf("write manifest %d: %v", index, err)
		}
	}
	selected, err := recoverySetAt(resolvedConfig{BackupDir: directory}, base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !selected.CreatedAt.Equal(base) {
		t.Fatalf("selected %s, want %s", selected.CreatedAt, base)
	}
}

func TestRecoveryReleaseVersionPrefersAuditedCurrentRelease(t *testing.T) {
	directory := t.TempDir()
	config := resolvedConfig{Environment: "production", EvidenceDir: directory}
	config.Application.APIVersion = "0.7.0-stale"
	if got := recoveryReleaseVersion(config); got != "0.7.0-stale" {
		t.Fatalf("fallback release version = %q", got)
	}
	release := releaseManifest{
		SchemaVersion: releaseSchemaVersion, ReleaseVersion: "0.7.0-current", Commit: strings.Repeat("a", 40),
		CreatedAt: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC), APIImage: "api", WebImage: "web",
		Migrations: []string{"migration"}, SBOMs: map[string]string{"api": "api.spdx.json", "web": "web.spdx.json"},
		Evidence: map[string]any{"ciRun": "test"},
	}
	if err := writeJSONAtomic(currentReleasePath(directory, "production"), release, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := recoveryReleaseVersion(config); got != "0.7.0-current" {
		t.Fatalf("current release version = %q", got)
	}
}

func TestRecoveryDrillGuardsProductionAndIsolation(t *testing.T) {
	app := NewApp(os.Stdout, os.Stderr)
	if _, err := app.recoveryDrill(context.Background(), resolvedConfig{Environment: "production"}); err == nil || !strings.Contains(err.Error(), "non-production") {
		t.Fatalf("production drill guard error = %v", err)
	}
	if _, err := app.recoveryDrill(context.Background(), resolvedConfig{Environment: "staging"}); err == nil || !strings.Contains(err.Error(), "HOTELMATE_ISOLATED_RECOVERY_DRILL") {
		t.Fatalf("isolation acknowledgement error = %v", err)
	}
}

func TestExtractUploadsArchiveRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "unsafe.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte("must not escape")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../escaped.txt", Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(directory, "restore")
	if err := extractUploadsArchive(archivePath, destination); err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("traversal error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("archive escaped destination: %v", err)
	}
}

func TestExtractUploadsArchivePreservesOperatorModes(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "uploads.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "check-in", Mode: 0o700, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	payload := []byte("private document")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "check-in/document.pdf", Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(directory, "restore")
	if err := extractUploadsArchive(archivePath, destination); err != nil {
		t.Fatal(err)
	}
	directoryInfo, err := os.Stat(filepath.Join(destination, "check-in"))
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != sharedUploadDirectoryMode {
		t.Fatalf("restored directory mode = %04o, want %04o", directoryInfo.Mode().Perm(), sharedUploadDirectoryMode)
	}
	fileInfo, err := os.Stat(filepath.Join(destination, "check-in", "document.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != sharedUploadFileMode {
		t.Fatalf("restored file mode = %04o, want %04o", fileInfo.Mode().Perm(), sharedUploadFileMode)
	}
}

func TestPostgresEnvironmentKeepsPasswordOutOfArguments(t *testing.T) {
	environment, err := postgresEnvironment("postgres://operator:s3cret@db.internal:55432/hotelmate?sslmode=require")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(environment, "\n")
	for _, expected := range []string{"PGHOST=db.internal", "PGPORT=55432", "PGUSER=operator", "PGPASSWORD=s3cret", "PGDATABASE=hotelmate", "PGSSLMODE=require"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in %q", expected, joined)
		}
	}
}

func TestReadRecoverySetRejectsArtifactTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hotelmate-20260823T000000Z.json")
	manifest := recoverySet{
		SchemaVersion:  recoverySchemaVersion,
		ID:             "hotelmate-20260823T000000Z",
		CreatedAt:      time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC),
		Environment:    "production",
		ReleaseVersion: "0.7.0-test",
		Database:       recoveryArtifact{File: "../outside.dump", SHA256: strings.Repeat("a", 64), Bytes: 2048},
		Migrations:     []database.MigrationStatus{{Version: "test", Applied: true}},
	}
	if err := writeJSONAtomic(path, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRecoverySet(path); err == nil || !strings.Contains(err.Error(), "base name") {
		t.Fatalf("artifact traversal error = %v", err)
	}
}

func TestResticSnapshotIDRequiresSummaryIdentity(t *testing.T) {
	valid := []byte("{\"message_type\":\"status\"}\n{\"message_type\":\"summary\",\"snapshot_id\":\"abc123\"}\n")
	if got := resticSnapshotID(valid); got != "abc123" {
		t.Fatalf("snapshot id = %q", got)
	}
	if got := resticSnapshotID([]byte(`{"message_type":"summary"}`)); got != "" {
		t.Fatalf("missing snapshot id accepted as %q", got)
	}
}
