package acceptance

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRejectsInvalidConfigurationBeforeNetworkAccess(t *testing.T) {
	t.Parallel()
	if _, err := Run(context.Background(), Options{BaseURL: "not-a-url", OnboardingToken: "token"}); err == nil || !strings.Contains(err.Error(), "absolute HTTP(S)") {
		t.Fatalf("expected URL validation error, got %v", err)
	}
	if _, err := Run(context.Background(), Options{BaseURL: "https://hotel.example"}); err == nil || !strings.Contains(err.Error(), "ACCEPTANCE_ONBOARDING_TOKEN") {
		t.Fatalf("expected onboarding-token validation error, got %v", err)
	}
}

func TestRunReturnsDeterministicTenantIdentifiersOnFirstRequestFailure(t *testing.T) {
	t.Parallel()
	result, err := Run(context.Background(), Options{
		BaseURL: "http://127.0.0.1:1", OnboardingToken: "token", Suffix: "suite",
		Now: func() time.Time { return time.Date(2026, 8, 23, 1, 2, 3, 0, time.UTC) },
	})
	if err == nil {
		t.Fatal("expected the unreachable target to fail")
	}
	if result.HotelSlug != "acceptance-20260823010203-suite" || result.OtherHotelSlug != "acceptance-other-20260823010203-suite" {
		t.Fatalf("unexpected identifiers: %#v", result)
	}
}

func TestResultSummary(t *testing.T) {
	t.Parallel()
	result := Result{BaseURL: "https://hotel.example", HotelSlug: "one", OtherHotelSlug: "two"}
	if got := result.Summary(); got != "HotelMate acceptance checks passed for https://hotel.example\nAcceptance tenants: one, two" {
		t.Fatalf("unexpected summary %q", got)
	}
}
