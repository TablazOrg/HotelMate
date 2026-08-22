package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type requestKey struct {
	Method, Pattern, Status string
}

type requestMetric struct {
	Count       uint64
	DurationSum float64
	Buckets     [6]uint64
}

var requestDurationBuckets = [...]float64{0.05, 0.1, 0.25, 0.5, 1, 2.5}

// Metrics is a dependency-free Prometheus text collector for the signals the
// single-replica deployment needs before adopting a larger telemetry SDK.
type Metrics struct {
	mu        sync.RWMutex
	requests  map[requestKey]requestMetric
	startedAt time.Time
	version   string
	commit    string
	image     string
	ready     atomic.Int64
	websocket atomic.Int64
}

func New(version, commit, image string) *Metrics {
	return &Metrics{requests: make(map[requestKey]requestMetric), startedAt: time.Now(), version: version, commit: commit, image: image}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &metricWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		key := requestKey{Method: r.Method, Pattern: pattern, Status: strconv.Itoa(status)}
		duration := time.Since(started).Seconds()
		m.mu.Lock()
		metric := m.requests[key]
		metric.Count++
		metric.DurationSum += duration
		for index, upperBound := range requestDurationBuckets {
			if duration <= upperBound {
				metric.Buckets[index]++
			}
		}
		m.requests[key] = metric
		m.mu.Unlock()
	})
}

func (m *Metrics) SetReady(ready bool) {
	if ready {
		m.ready.Store(1)
		return
	}
	m.ready.Store(0)
}

func (m *Metrics) WebsocketOpened() { m.websocket.Add(1) }
func (m *Metrics) WebsocketClosed() { m.websocket.Add(-1) }

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.mu.RLock()
		keys := make([]requestKey, 0, len(m.requests))
		for key := range m.requests {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Method+keys[i].Pattern+keys[i].Status < keys[j].Method+keys[j].Pattern+keys[j].Status
		})
		metrics := make(map[requestKey]requestMetric, len(m.requests))
		for _, key := range keys {
			metrics[key] = m.requests[key]
		}
		m.mu.RUnlock()

		_, _ = fmt.Fprintln(w, "# HELP hotelmate_build_info Release identity for the running API.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_build_info gauge")
		_, _ = fmt.Fprintf(w, "hotelmate_build_info{version=%q,commit=%q,image=%q} 1\n", escapeLabel(m.version), escapeLabel(m.commit), escapeLabel(m.image))
		_, _ = fmt.Fprintln(w, "# HELP hotelmate_uptime_seconds Seconds since the API process started.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_uptime_seconds gauge")
		_, _ = fmt.Fprintf(w, "hotelmate_uptime_seconds %.3f\n", time.Since(m.startedAt).Seconds())
		_, _ = fmt.Fprintln(w, "# HELP hotelmate_ready Database-backed readiness state.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_ready gauge")
		_, _ = fmt.Fprintf(w, "hotelmate_ready %d\n", m.ready.Load())
		_, _ = fmt.Fprintln(w, "# HELP hotelmate_websocket_connections Active realtime connections.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_websocket_connections gauge")
		_, _ = fmt.Fprintf(w, "hotelmate_websocket_connections %d\n", m.websocket.Load())
		_, _ = fmt.Fprintln(w, "# HELP hotelmate_http_requests_total HTTP requests by method, route pattern, and status.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_http_requests_total counter")
		_, _ = fmt.Fprintln(w, "# HELP hotelmate_http_request_duration_seconds HTTP request duration histogram.")
		_, _ = fmt.Fprintln(w, "# TYPE hotelmate_http_request_duration_seconds histogram")
		for _, key := range keys {
			metric := metrics[key]
			labels := fmt.Sprintf("method=%q,pattern=%q,status=%q", escapeLabel(key.Method), escapeLabel(key.Pattern), escapeLabel(key.Status))
			_, _ = fmt.Fprintf(w, "hotelmate_http_requests_total{%s} %d\n", labels, metric.Count)
			for index, upperBound := range requestDurationBuckets {
				_, _ = fmt.Fprintf(w, "hotelmate_http_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, strconv.FormatFloat(upperBound, 'f', -1, 64), metric.Buckets[index])
			}
			_, _ = fmt.Fprintf(w, "hotelmate_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, metric.Count)
			_, _ = fmt.Fprintf(w, "hotelmate_http_request_duration_seconds_sum{%s} %.6f\n", labels, metric.DurationSum)
			_, _ = fmt.Fprintf(w, "hotelmate_http_request_duration_seconds_count{%s} %d\n", labels, metric.Count)
		}
	})
}

type metricWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *metricWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *metricWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\"", "\\\"")
}
