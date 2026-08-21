package config

import (
	"os"
	"strconv"
	"strings"
)

// Config contains process-level configuration. Values are intentionally read
// from the environment so local, container, and hosted deployments share the
// same application binary.
type Config struct {
	Environment    string
	HTTPAddr       string
	APIVersion     string
	DatabaseURL    string
	JWTSecret      string
	AllowedOrigins []string
	AutoMigrate    bool
}

func Load() Config {
	return Config{
		Environment:    envOrDefault("APP_ENV", "development"),
		HTTPAddr:       envOrDefault("API_HTTP_ADDR", ":8080"),
		APIVersion:     envOrDefault("API_VERSION", "0.1.0"),
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://hotelmate:hotelmate@localhost:5432/hotelmate?sslmode=disable"),
		JWTSecret:      envOrDefault("JWT_SECRET", "replace-this-development-secret"),
		AllowedOrigins: csvEnv("ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		AutoMigrate:    boolEnv("AUTO_MIGRATE", true),
	}
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
