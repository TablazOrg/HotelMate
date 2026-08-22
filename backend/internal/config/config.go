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
	return Config{
		Environment:       envOrDefault("APP_ENV", "development"),
		HTTPAddr:          envOrDefault("API_HTTP_ADDR", ":8080"),
		APIVersion:        envOrDefault("API_VERSION", "1.0.0"),
		DatabaseURL:       envOrDefault("DATABASE_URL", "postgres://hotelmate:hotelmate@localhost:5432/hotelmate?sslmode=disable"),
		JWTSecret:         envOrDefault("JWT_SECRET", "replace-this-development-secret-now"),
		JWTIssuer:         envOrDefault("JWT_ISSUER", "hotelmate-api"),
		StaffTokenTTL:     durationEnv("STAFF_TOKEN_TTL", 8*time.Hour),
		GuestTokenTTL:     durationEnv("GUEST_TOKEN_TTL", 24*time.Hour),
		OnboardingToken:   envOrDefault("ONBOARDING_TOKEN", "replace-this-onboarding-token"),
		UploadsDir:        envOrDefault("UPLOADS_DIR", "uploads"),
		DocumentMaxBytes:  int64Env("DOCUMENT_MAX_BYTES", 5*1024*1024),
		DocumentRetention: durationEnv("DOCUMENT_RETENTION", 720*time.Hour),
		ChatRetention:     durationEnv("CHAT_RETENTION", 2160*time.Hour),
		ChatConfidence:    float64Env("CHAT_CONFIDENCE_THRESHOLD", 0.5),
		AllowedOrigins:    csvEnv("ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		AutoMigrate:       boolEnv("AUTO_MIGRATE", true),
		EnableHSTS:        boolEnv("ENABLE_HSTS", strings.EqualFold(envOrDefault("APP_ENV", "development"), "production")),
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

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
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

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func float64Env(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
