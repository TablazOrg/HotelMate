package operations

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/TablazOrg/HotelMate/backend/internal/config"
)

type cliOptions struct {
	json, yes, dryRun, help bool
	values                  map[string]string
}

type resolvedConfig struct {
	Application      appconfig.Config
	Environment      string
	DatabaseURL      string
	UploadsDir       string
	BackupDir        string
	BaseURL          string
	ComposeFile      string
	EnvFile          string
	ReleaseFile      string
	EvidenceDir      string
	ManifestFile     string
	ResticRepository string
	ResticPassword   string
	ResticKeepDaily  int
	ResticKeepWeekly int
	MinDiskBytes     uint64
	TextfileDir      string
	PostgresDriver   string
	CosignIdentity   string
	CosignIssuer     string
	OffHostDeferred  bool
	RecoveryDrill    bool
	RecoveryPoint    string
	DrillOperator    string
	DrillMaxRPO      time.Duration
	DrillMaxRTO      time.Duration
	lookup           func(string) string
}

var valueFlags = map[string]string{
	"--config": "CONFIG_FILE", "--environment": "APP_ENV", "--database-url": "DATABASE_URL",
	"--uploads-dir": "UPLOADS_DIR", "--backup-dir": "HOTELMATE_BACKUP_DIR", "--directory": "HOTELMATE_BACKUP_DIR",
	"--base-url": "HOTELMATE_BASE_URL", "--compose-file": "HOTELMATE_COMPOSE_FILE", "--env-file": "HOTELMATE_ENV_FILE",
	"--release-file": "HOTELMATE_RELEASE_FILE", "--evidence-dir": "HOTELMATE_EVIDENCE_DIR",
	"--manifest": "HOTELMATE_BACKUP_MANIFEST", "--file": "HOTELMATE_BACKUP_MANIFEST",
	"--requested-recovery-point": "HOTELMATE_RECOVERY_POINT", "--operator": "HOTELMATE_OPERATOR",
}

func parseOptions(args []string) (cliOptions, []string, error) {
	options := cliOptions{values: map[string]string{}}
	path := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			options.json = true
		case "--yes", "--confirm":
			options.yes = true
		case "--dry-run":
			options.dryRun = true
		case "--help", "-h":
			options.help = true
		default:
			name, value, hasValue := strings.Cut(arg, "=")
			key, known := valueFlags[name]
			if !known {
				if strings.HasPrefix(arg, "-") {
					return options, path, failure(ExitInvalid, fmt.Sprintf("unknown flag %q", arg), nil)
				}
				path = append(path, arg)
				continue
			}
			if !hasValue {
				index++
				if index >= len(args) {
					return options, path, failure(ExitInvalid, fmt.Sprintf("%s requires a value", name), nil)
				}
				value = args[index]
			}
			if strings.TrimSpace(value) == "" {
				return options, path, failure(ExitInvalid, fmt.Sprintf("%s cannot be empty", name), nil)
			}
			options.values[key] = value
		}
	}
	return options, path, nil
}

func (a *App) resolveConfig(options cliOptions) (resolvedConfig, error) {
	fileValues := map[string]string{}
	if path := options.values["CONFIG_FILE"]; path != "" {
		values, err := readProtectedEnvFile(path)
		if err != nil {
			return resolvedConfig{}, err
		}
		fileValues = values
	}
	lookup := func(key string) string {
		if value := strings.TrimSpace(options.values[key]); value != "" {
			return value
		}
		if value := strings.TrimSpace(a.Getenv(key)); value != "" {
			return value
		}
		return strings.TrimSpace(fileValues[key])
	}
	environment := firstValue(lookup("APP_ENV"), lookup("HOTELMATE_ENVIRONMENT"), "development")
	defaultCompose := "docker-compose.yml"
	defaultEnvFile := ".env"
	if environment == "staging" || environment == "production" {
		defaultCompose = "docker-compose.production.yml"
		defaultEnvFile = ".env." + environment
	}
	keepDaily, _ := strconv.Atoi(firstValue(lookup("RESTIC_KEEP_DAILY"), "14"))
	keepWeekly, _ := strconv.Atoi(firstValue(lookup("RESTIC_KEEP_WEEKLY"), "8"))
	minDisk, _ := strconv.ParseUint(firstValue(lookup("HOTELMATE_MIN_DISK_BYTES"), "2147483648"), 10, 64)
	maxRPO, err := optionalDuration(lookup("HOTELMATE_DRILL_MAX_RPO"))
	if err != nil {
		return resolvedConfig{}, err
	}
	maxRTO, err := optionalDuration(lookup("HOTELMATE_DRILL_MAX_RTO"))
	if err != nil {
		return resolvedConfig{}, err
	}
	recoveryDrill, _ := strconv.ParseBool(lookup("HOTELMATE_ISOLATED_RECOVERY_DRILL"))
	offHostDeferred, _ := strconv.ParseBool(lookup("HOTELMATE_OWNER_APPROVED_OFFHOST_BACKUP_DEFERRAL"))
	resolved := resolvedConfig{
		Environment: environment, DatabaseURL: lookup("DATABASE_URL"), UploadsDir: firstValue(lookup("UPLOADS_DIR"), "uploads"),
		BackupDir:    firstValue(lookup("HOTELMATE_BACKUP_DIR"), filepath.Join(a.WorkingDir, ".hotelmate", "backups")),
		BaseURL:      firstValue(lookup("HOTELMATE_BASE_URL"), "http://localhost:3000"),
		ComposeFile:  firstValue(lookup("HOTELMATE_COMPOSE_FILE"), filepath.Join(a.WorkingDir, defaultCompose)),
		EnvFile:      firstValue(lookup("HOTELMATE_ENV_FILE"), filepath.Join(a.WorkingDir, defaultEnvFile)),
		ReleaseFile:  firstValue(lookup("HOTELMATE_RELEASE_FILE"), filepath.Join(a.WorkingDir, ".hotelmate", "release.json")),
		EvidenceDir:  firstValue(lookup("HOTELMATE_EVIDENCE_DIR"), filepath.Join(a.WorkingDir, ".hotelmate", "evidence")),
		ManifestFile: lookup("HOTELMATE_BACKUP_MANIFEST"), ResticRepository: lookup("RESTIC_REPOSITORY"),
		ResticPassword: lookup("RESTIC_PASSWORD"), ResticKeepDaily: keepDaily, ResticKeepWeekly: keepWeekly, MinDiskBytes: minDisk,
		TextfileDir:     lookup("HOTELMATE_TEXTFILE_DIR"),
		PostgresDriver:  firstValue(lookup("HOTELMATE_POSTGRES_DRIVER"), "direct"),
		CosignIdentity:  lookup("COSIGN_CERTIFICATE_IDENTITY"),
		CosignIssuer:    firstValue(lookup("COSIGN_CERTIFICATE_OIDC_ISSUER"), "https://token.actions.githubusercontent.com"),
		OffHostDeferred: offHostDeferred,
		RecoveryDrill:   recoveryDrill,
		RecoveryPoint:   lookup("HOTELMATE_RECOVERY_POINT"),
		DrillOperator:   firstValue(lookup("HOTELMATE_OPERATOR"), lookup("USER"), "unknown"),
		DrillMaxRPO:     maxRPO,
		DrillMaxRTO:     maxRTO,
		lookup:          lookup,
	}
	resolved.Application = appconfig.LoadFrom(func(key string) string {
		if key == "APP_ENV" {
			return environment
		}
		return lookup(key)
	})
	resolved.DatabaseURL = resolved.Application.DatabaseURL
	resolved.UploadsDir = firstValue(lookup("HOTELMATE_HOST_UPLOADS_DIR"), resolved.Application.UploadsDir)
	return resolved, nil
}

func optionalDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("expected a positive duration, got %q", value)
	}
	return duration, nil
}

func (a *App) validateConfig(config resolvedConfig) (any, error) {
	if config.Environment != "development" && config.Environment != "staging" && config.Environment != "production" {
		return nil, failure(ExitInvalid, "APP_ENV must be development, staging, or production", nil)
	}
	if err := config.Application.Validate(); err != nil {
		return nil, failure(ExitInvalid, "application configuration is invalid", err)
	}
	if parsed, err := url.Parse(config.BaseURL); err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, failure(ExitInvalid, "HOTELMATE_BASE_URL must be an absolute HTTP(S) URL", err)
	}
	if config.ResticRepository != "" && config.ResticPassword == "" {
		return nil, failure(ExitInvalid, "RESTIC_PASSWORD is required when RESTIC_REPOSITORY is configured", nil)
	}
	if config.PostgresDriver != "direct" && config.PostgresDriver != "compose" {
		return nil, failure(ExitInvalid, "HOTELMATE_POSTGRES_DRIVER must be direct or compose", nil)
	}
	if config.Environment == "staging" || config.Environment == "production" {
		missing := make([]string, 0, 3)
		for _, key := range []string{"SMOKE_HOTEL_SLUG", "SMOKE_STAFF_EMAIL", "SMOKE_STAFF_PASSWORD"} {
			if config.lookup(key) == "" {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return nil, failure(ExitInvalid, "authenticated smoke configuration is incomplete: "+strings.Join(missing, ", "), nil)
		}
	}
	return map[string]any{
		"environment": config.Environment, "databaseConfigured": config.DatabaseURL != "", "uploadsDir": config.UploadsDir,
		"backupDir": config.BackupDir, "baseURL": config.BaseURL, "offHostBackupConfigured": config.ResticRepository != "",
		"offHostBackupOwnerDeferred": config.OffHostDeferred,
		"secretsRedacted":            true,
	}, nil
}

func readProtectedEnvFile(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("config file must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("config file %s must not be accessible by group or others (use chmod 600)", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid config line %d", lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, scanner.Err()
}

func firstValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
