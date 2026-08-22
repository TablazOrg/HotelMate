package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains process-level configuration. Values are intentionally read
// from the environment so local, container, and hosted deployments share the
// same application binary.
type Config struct {
	Environment       string
	HTTPAddr          string
	APIVersion        string
	ReleaseCommit     string
	ReleaseImage      string
	DatabaseURL       string
	JWTSecret         string
	JWTIssuer         string
	StaffTokenTTL     time.Duration
	GuestTokenTTL     time.Duration
	OnboardingToken   string
	UploadsDir        string
	DocumentMaxBytes  int64
	DocumentRetention time.Duration
	ChatRetention     time.Duration
	ChatConfidence    float64
	AllowedOrigins    []string
	AutoMigrate       bool
	EnableHSTS        bool
}

func Load() Config {
	return LoadFrom(os.Getenv)
}

// LoadFrom resolves application configuration with the supplied lookup
// function. The operations CLI uses this to apply the documented precedence
// of flags, environment variables, and a protected config file without
// mutating the process environment.
func LoadFrom(getenv func(string) string) Config {
	return Config{
		Environment:       envOrDefault(getenv, "APP_ENV", "development"),
		HTTPAddr:          envOrDefault(getenv, "API_HTTP_ADDR", ":8080"),
		APIVersion:        envOrDefault(getenv, "API_VERSION", "1.0.0"),
		ReleaseCommit:     envOrDefault(getenv, "RELEASE_COMMIT", "unknown"),
		ReleaseImage:      envOrDefault(getenv, "RELEASE_IMAGE", "unknown"),
		DatabaseURL:       envOrDefault(getenv, "DATABASE_URL", "postgres://hotelmate:hotelmate@127.0.0.1:5432/hotelmate?sslmode=disable"),
		JWTSecret:         envOrDefault(getenv, "JWT_SECRET", "replace-this-development-secret-now"),
		JWTIssuer:         envOrDefault(getenv, "JWT_ISSUER", "hotelmate-api"),
		StaffTokenTTL:     durationEnv(getenv, "STAFF_TOKEN_TTL", 8*time.Hour),
		GuestTokenTTL:     durationEnv(getenv, "GUEST_TOKEN_TTL", 24*time.Hour),
		OnboardingToken:   envOrDefault(getenv, "ONBOARDING_TOKEN", "replace-this-onboarding-token"),
		UploadsDir:        envOrDefault(getenv, "UPLOADS_DIR", "uploads"),
		DocumentMaxBytes:  int64Env(getenv, "DOCUMENT_MAX_BYTES", 5*1024*1024),
		DocumentRetention: durationEnv(getenv, "DOCUMENT_RETENTION", 720*time.Hour),
		ChatRetention:     durationEnv(getenv, "CHAT_RETENTION", 2160*time.Hour),
		ChatConfidence:    float64Env(getenv, "CHAT_CONFIDENCE_THRESHOLD", 0.5),
		AllowedOrigins:    csvEnv(getenv, "ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		AutoMigrate:       boolEnv(getenv, "AUTO_MIGRATE", true),
		EnableHSTS:        boolEnv(getenv, "ENABLE_HSTS", strings.EqualFold(envOrDefault(getenv, "APP_ENV", "development"), "production")),
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	if c.StaffTokenTTL <= 0 || c.GuestTokenTTL <= 0 {
		return fmt.Errorf("token TTL values must be positive")
	}
	if strings.TrimSpace(c.UploadsDir) == "" || c.DocumentMaxBytes < 1024 || c.DocumentRetention <= 0 {
		return fmt.Errorf("document storage configuration is invalid")
	}
	if c.ChatRetention <= 0 || c.ChatConfidence <= 0 || c.ChatConfidence > 1 {
		return fmt.Errorf("chat safety configuration is invalid")
	}
	if c.Environment == "production" {
		if strings.HasPrefix(c.JWTSecret, "replace-") {
			return fmt.Errorf("JWT_SECRET must be changed in production")
		}
		if len(c.OnboardingToken) < 24 || strings.HasPrefix(c.OnboardingToken, "replace-") {
			return fmt.Errorf("ONBOARDING_TOKEN must be a production secret of at least 24 characters")
		}
	}
	return nil
}

func envOrDefault(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(getenv func(string) string, key string, fallback []string) []string {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func boolEnv(getenv func(string) string, key string, fallback bool) bool {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(getenv func(string) string, key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func int64Env(getenv func(string) string, key string, fallback int64) int64 {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func float64Env(getenv func(string) string, key string, fallback float64) float64 {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
