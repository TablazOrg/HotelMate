package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

type executorFunc func(context.Context, string, []string, []string, io.Reader, io.Writer, io.Writer) error

func (f executorFunc) Run(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return f(ctx, name, args, env, stdin, stdout, stderr)
}

func TestJSONVersionContract(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)
	app.Version = "7.0.0"
	app.Commit = "abc123"
	app.Now = func() time.Time { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) }
	if code := app.Run(context.Background(), []string{"--json", "version"}); code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != SchemaVersion || envelope.Command != "version" || !envelope.OK {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestConfigPrecedenceAndSecretRedaction(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "hotelmate.env")
	contents := strings.Join([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://file-user:file-secret@file-db/hotelmate",
		"JWT_SECRET=file-jwt-secret-that-is-longer-than-thirty-two-characters",
		"ONBOARDING_TOKEN=file-onboarding-secret-long-enough",
		"ALLOWED_ORIGINS=https://file.example.com",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{
		"DATABASE_URL":         "postgres://env-user:env-secret@env-db/hotelmate",
		"JWT_SECRET":           "env-jwt-secret-that-is-longer-than-thirty-two-characters",
		"ONBOARDING_TOKEN":     "env-onboarding-secret-long-enough",
		"ALLOWED_ORIGINS":      "https://env.example.com",
		"SMOKE_HOTEL_SLUG":     "smoke-hotel",
		"SMOKE_STAFF_EMAIL":    "smoke@example.com",
		"SMOKE_STAFF_PASSWORD": "smoke-password",
	}
	var stdout, stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)
	app.Getenv = func(key string) string { return environment[key] }
	flagURL := "postgres://flag-user:flag-secret@flag-db/hotelmate"
	code := app.Run(context.Background(), []string{"--json", "--config", configPath, "--database-url", flagURL, "config", "validate"})
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	combined := stdout.String() + stderr.String()
	for _, secret := range []string{"file-secret", "env-secret", "flag-secret", environment["JWT_SECRET"], environment["ONBOARDING_TOKEN"], environment["SMOKE_STAFF_PASSWORD"]} {
		if strings.Contains(combined, secret) {
			t.Fatalf("output leaked secret %q: %s", secret, combined)
		}
	}
}

func TestConfigFileMustBeProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "open.env")
	if err := os.WriteFile(path, []byte("APP_ENV=development\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)
	if code := app.Run(context.Background(), []string{"--config", path, "config", "validate"}); code != ExitInvalid {
		t.Fatalf("exit code = %d, want %d", code, ExitInvalid)
	}
	if !strings.Contains(stderr.String(), "chmod 600") {
		t.Fatalf("missing mode diagnostic: %s", stderr.String())
	}
}

func TestSmokeSuccessAndVerificationFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/readyz":
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case "/api/v1":
			_, _ = w.Write([]byte(`{"name":"HotelMate API"}`))
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)
	if code := app.Run(context.Background(), []string{"--base-url", server.URL, "smoke"}); code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	server.Close()
	stdout.Reset()
	stderr.Reset()
	if code := app.Run(context.Background(), []string{"--base-url", server.URL, "smoke"}); code != ExitVerification {
		t.Fatalf("exit code = %d, want %d", code, ExitVerification)
	}
}

func TestMutationsRequireConfirmationBeforeExternalAccess(t *testing.T) {
	commands := [][]string{
		{"migrate", "up"},
		{"backup", "create"},
		{"backup", "restore"},
		{"backup", "drill"},
		{"retention", "purge-documents"},
		{"retention", "purge-messages"},
		{"deploy", "apply"},
		{"deploy", "rollback"},
	}
	for _, command := range commands {
		t.Run(strings.Join(command, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			app := NewApp(&stdout, &stderr)
			app.Getenv = func(string) string { return "" }
			app.Executor = executorFunc(func(context.Context, string, []string, []string, io.Reader, io.Writer, io.Writer) error {
				t.Fatal("external command ran before confirmation")
				return nil
			})
			code := app.Run(context.Background(), command)
			if code != ExitPrecondition || !strings.Contains(stderr.String(), "requires --yes") {
				t.Fatalf("exit=%d stderr=%s", code, stderr.String())
			}
		})
	}
}

func TestBackupVerificationAndChecksumFailure(t *testing.T) {
	directory := t.TempDir()
	dumpPath := filepath.Join(directory, "hotelmate-20260822T120000Z.dump")
	if err := os.WriteFile(dumpPath, bytes.Repeat([]byte("database"), 256), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact, err := inspectArtifact(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "hotelmate-20260822T120000Z.json")
	manifest := recoverySet{
		SchemaVersion:  recoverySchemaVersion,
		ID:             "hotelmate-20260822T120000Z",
		CreatedAt:      time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
		Environment:    "production",
		ReleaseVersion: "0.7.0-test",
		Database:       artifact,
		Migrations:     []database.MigrationStatus{{Version: "test", Applied: true}},
	}
	if err := writeJSONAtomic(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	newApp := func(stdout, stderr *bytes.Buffer) *App {
		app := NewApp(stdout, stderr)
		app.Getenv = func(string) string { return "" }
		app.LookPath = func(name string) (string, error) {
			if name == "pg_restore" {
				return "/usr/bin/pg_restore", nil
			}
			return "", errors.New("unexpected tool")
		}
		app.Executor = executorFunc(func(_ context.Context, name string, _ []string, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			if name != "pg_restore" {
				return errors.New("unexpected external command")
			}
			_, _ = io.WriteString(stdout, "; archive header\n1; 0 0 TABLE public hotels hotelmate\n")
			return nil
		})
		return app
	}

	var stdout, stderr bytes.Buffer
	app := newApp(&stdout, &stderr)
	if code := app.Run(context.Background(), []string{"--manifest", manifestPath, "backup", "verify"}); code != ExitOK {
		t.Fatalf("verification exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"catalogEntries": 1`) {
		t.Fatalf("missing catalog evidence: %s", stdout.String())
	}

	if err := os.WriteFile(dumpPath, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	app = newApp(&stdout, &stderr)
	if code := app.Run(context.Background(), []string{"--manifest", manifestPath, "backup", "verify"}); code != ExitVerification {
		t.Fatalf("tampered verification exit=%d stderr=%s", code, stderr.String())
	}
}

func TestJobMetricFailurePreservesLastSuccess(t *testing.T) {
	directory := t.TempDir()
	succeededAt := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	failedAt := succeededAt.Add(time.Hour)
	if err := recordJobMetric(directory, "backup", true, succeededAt); err != nil {
		t.Fatal(err)
	}
	if err := recordJobMetric(directory, "backup", false, failedAt); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(directory, "hotelmate_backup.prom"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{
		`hotelmate_job_last_run_success{job="backup"} 0`,
		`hotelmate_job_last_run_timestamp_seconds{job="backup"} ` + fmt.Sprint(failedAt.Unix()),
		`hotelmate_job_last_success_timestamp_seconds{job="backup"} ` + fmt.Sprint(succeededAt.Unix()),
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("metric output missing %q:\n%s", expected, text)
		}
	}
}

func TestMergeEnvironmentReplacesSecretsInsteadOfDuplicatingThem(t *testing.T) {
	merged := mergeEnvironment(
		[]string{"PGPASSWORD=old-secret", "PATH=/usr/bin", "PGPASSWORD=older-secret"},
		[]string{"PGPASSWORD=new-secret", "RESTIC_PASSWORD=restic-secret"},
	)
	joined := strings.Join(merged, "\n")
	if strings.Contains(joined, "old-secret") || strings.Count(joined, "PGPASSWORD=") != 1 {
		t.Fatalf("base secret was not replaced: %q", joined)
	}
	for _, expected := range []string{"PATH=/usr/bin", "PGPASSWORD=new-secret", "RESTIC_PASSWORD=restic-secret"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("merged environment missing %q: %q", expected, joined)
		}
	}
}
