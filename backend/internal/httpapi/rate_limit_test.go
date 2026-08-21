package httpapi

import (
	"testing"
	"time"
)

func TestIPRateLimiterResetsAfterWindow(t *testing.T) {
	limiter := newIPRateLimiter(2, time.Minute)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	if !limiter.allow("client") || !limiter.allow("client") {
		t.Fatal("first two requests should be allowed")
	}
	if limiter.allow("client") {
		t.Fatal("third request should be limited")
	}
	now = now.Add(time.Minute)
	if !limiter.allow("client") {
		t.Fatal("request should be allowed after window resets")
	}
}
