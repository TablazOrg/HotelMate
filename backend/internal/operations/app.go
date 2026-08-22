package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

const SchemaVersion = "hotelmate.operations/v1"

const (
	ExitOK           = 0
	ExitInvalid      = 2
	ExitPrecondition = 3
	ExitCommand      = 4
	ExitVerification = 5
)

type commandError struct {
	Code    int
	Message string
	Cause   error
}

func (e *commandError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func failure(code int, message string, cause error) error {
	return &commandError{Code: code, Message: message, Cause: cause}
}

type Envelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Command       string `json:"command"`
	OK            bool   `json:"ok"`
	Timestamp     string `json:"timestamp"`
	Data          any    `json:"data,omitempty"`
	Error         string `json:"error,omitempty"`
}

type Executor interface {
	Run(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = mergeEnvironment(os.Environ(), env)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func mergeEnvironment(base, overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		if key, _, ok := strings.Cut(item, "="); ok {
			keys[key] = struct{}{}
		}
	}
	merged := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if _, replaced := keys[key]; ok && replaced {
			continue
		}
		merged = append(merged, item)
	}
	return append(merged, overrides...)
}

type App struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Getenv     func(string) string
	LookPath   func(string) (string, error)
	Executor   Executor
	Now        func() time.Time
	WorkingDir string
	Version    string
	Commit     string
	BuildDate  string
}

func NewApp(stdout, stderr io.Writer) *App {
	workingDir, _ := os.Getwd()
	workingDir = findRepositoryRoot(workingDir)
	return &App{
		Stdout: stdout, Stderr: stderr, Getenv: os.Getenv, LookPath: exec.LookPath,
		Executor: OSExecutor{}, Now: time.Now, WorkingDir: workingDir,
		Version: "dev", Commit: "unknown", BuildDate: "unknown",
	}
}

func findRepositoryRoot(start string) string {
	candidate := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(candidate, "docker-compose.yml")); err == nil {
			return candidate
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return start
		}
		candidate = parent
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	options, path, err := parseOptions(args)
	if err != nil {
		return a.emit(options, strings.Join(path, " "), nil, err)
	}
	command := strings.Join(path, " ")
	if len(path) == 0 || path[0] == "help" || options.help {
		return a.emit(options, "help", map[string]any{"usage": usage()}, nil)
	}

	resolved, err := a.resolveConfig(options)
	if err != nil {
		return a.emit(options, command, nil, failure(ExitInvalid, "configuration resolution failed", err))
	}

	var data any
	switch path[0] {
	case "version":
		data = map[string]any{"version": a.Version, "commit": a.Commit, "buildDate": a.BuildDate, "migrations": database.MigrationVersions()}
	case "config":
		if len(path) != 2 || path[1] != "validate" {
			err = failure(ExitInvalid, "usage: hotelmate config validate", nil)
		} else {
			data, err = a.validateConfig(resolved)
		}
	case "doctor":
		data, err = a.doctor(ctx, resolved)
	case "migrate":
		data, err = a.migrate(ctx, resolved, path, options)
	case "backup":
		data, err = a.backup(ctx, resolved, path, options)
	case "retention":
		data, err = a.retention(ctx, resolved, path, options)
	case "deploy":
		data, err = a.deploy(ctx, resolved, path, options)
	case "smoke":
		data, err = a.smoke(ctx, resolved)
	case "acceptance":
		data, err = a.acceptance(ctx, resolved)
	default:
		err = failure(ExitInvalid, fmt.Sprintf("unknown command %q", path[0]), nil)
	}
	if job := metricJob(path, options); job != "" {
		if metricErr := recordJobMetric(resolved.TextfileDir, job, err == nil, a.Now().UTC()); metricErr != nil && err == nil {
			err = failure(ExitVerification, "record operation monitoring evidence failed", metricErr)
		}
	}
	return a.emit(options, command, data, err)
}

func metricJob(path []string, options cliOptions) string {
	if options.dryRun || len(path) != 2 {
		return ""
	}
	switch strings.Join(path, " ") {
	case "migrate up":
		return "migrate"
	case "backup create":
		return "backup"
	case "backup restore":
		return "restore"
	case "backup drill":
		return "restore_drill"
	case "retention purge-documents":
		return "purge_documents"
	case "retention purge-messages":
		return "purge_messages"
	case "deploy apply":
		return "deploy"
	case "deploy rollback":
		return "rollback"
	default:
		return ""
	}
}

func (a *App) emit(options cliOptions, command string, data any, err error) int {
	code := ExitOK
	message := ""
	if err != nil {
		message = err.Error()
		var typed *commandError
		if errors.As(err, &typed) {
			code = typed.Code
		} else {
			code = ExitCommand
		}
		fmt.Fprintln(a.Stderr, message)
	}
	if options.json {
		_ = json.NewEncoder(a.Stdout).Encode(Envelope{
			SchemaVersion: SchemaVersion, Command: command, OK: err == nil,
			Timestamp: a.Now().UTC().Format(time.RFC3339), Data: data, Error: message,
		})
	} else if err == nil {
		if command == "help" {
			fmt.Fprint(a.Stdout, usage())
		} else {
			printHuman(a.Stdout, data)
		}
	}
	return code
}

func printHuman(output io.Writer, data any) {
	switch value := data.(type) {
	case nil:
		return
	case string:
		fmt.Fprintln(output, value)
	case map[string]string:
		for _, key := range []string{"version", "commit", "buildDate"} {
			if item := value[key]; item != "" {
				fmt.Fprintf(output, "%s: %s\n", key, item)
			}
		}
	default:
		encoded, _ := json.MarshalIndent(data, "", "  ")
		fmt.Fprintln(output, string(encoded))
	}
}

func usage() string {
	return `HotelMate operations CLI

Usage:
  hotelmate [global flags] doctor
  hotelmate [global flags] config validate
  hotelmate [global flags] migrate status|up [--dry-run]
  hotelmate [global flags] backup create|list|verify|restore|drill
  hotelmate [global flags] retention purge-documents|purge-messages
  hotelmate [global flags] deploy preflight|apply|status|rollback
  hotelmate [global flags] smoke
  hotelmate [global flags] acceptance
  hotelmate [global flags] version

Global flags:
  --json                       emit hotelmate.operations/v1 JSON
  --config PATH                protected KEY=value configuration file
  --environment NAME           development, staging, or production
  --database-url URL           overrides DATABASE_URL (never printed)
  --uploads-dir PATH           private document directory
  --backup-dir PATH            backup catalog directory
  --base-url URL               smoke/acceptance target
  --compose-file PATH          deployment Compose file
  --env-file PATH              deployment environment file
  --release-file PATH          immutable release manifest
  --evidence-dir PATH          deployment evidence directory
  --manifest PATH              backup manifest for verify/restore
  --requested-recovery-point  RFC3339 point used to measure drill RPO
  --operator NAME             recovery-drill operator identity
  --yes, --confirm             authorize a mutating command
  --dry-run                    report planned migration/deployment work
`
}

func (a *App) repoPath(parts ...string) string {
	items := append([]string{a.WorkingDir}, parts...)
	return filepath.Clean(filepath.Join(items...))
}
