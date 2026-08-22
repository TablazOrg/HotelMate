package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsExposeReleaseIdentityAndBoundedRoutePattern(t *testing.T) {
	metrics := New("0.7.0", "abc123", "registry.example/api@sha256:deadbeef")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hotels/{hotelID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/hotels/tenant-specific-value", nil)
	response := httptest.NewRecorder()
	metrics.Middleware(mux).ServeHTTP(response, request)

	metricResponse := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricResponse.Body.String()
	for _, expected := range []string{
		`hotelmate_build_info{version="0.7.0",commit="abc123",image="registry.example/api@sha256:deadbeef"} 1`,
		`pattern="GET /hotels/{hotelID}"`,
		`status="204"`,
		`hotelmate_http_request_duration_seconds_bucket`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "tenant-specific-value") {
		t.Fatalf("raw route value leaked into metric labels: %s", body)
	}
}
