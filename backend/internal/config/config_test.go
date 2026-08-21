package config

import (
	"testing"
	"time"
)

func TestProductionRejectsPlaceholderSecrets(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseURL: "postgres://example", JWTSecret: "replace-this-development-secret-now",
		JWTIssuer: "hotelmate", StaffTokenTTL: time.Hour, GuestTokenTTL: time.Hour, OnboardingToken: "replace-this-onboarding-token",
		UploadsDir: "/app/uploads", DocumentMaxBytes: 1024, DocumentRetention: time.Hour,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("production placeholder secrets must be rejected")
	}
}

func TestValidProductionConfig(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseURL: "postgres://example", JWTSecret: "a-secure-jwt-secret-with-more-than-32-characters",
		JWTIssuer: "hotelmate", StaffTokenTTL: time.Hour, GuestTokenTTL: time.Hour,
		OnboardingToken: "a-separate-secure-onboarding-token",
		UploadsDir:      "/app/uploads", DocumentMaxBytes: 1024, DocumentRetention: time.Hour,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid production config: %v", err)
	}
}
