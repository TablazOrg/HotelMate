package httpapi

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	count   int
	resetAt time.Time
}

type ipRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]rateWindow
	now     func() time.Time
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{limit: limit, window: window, clients: make(map[string]rateWindow), now: time.Now}
}

func (l *ipRateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	current, exists := l.clients[key]
	if !exists && len(l.clients) >= 10000 {
		for client, candidate := range l.clients {
			if !now.Before(candidate.resetAt) {
				delete(l.clients, client)
			}
		}
		if len(l.clients) >= 10000 {
			return false
		}
	}
	if !exists || !now.Before(current.resetAt) {
		l.clients[key] = rateWindow{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if current.count >= l.limit {
		return false
	}
	current.count++
	l.clients[key] = current
	return true
}

func (s *Server) limit(limiter *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r) + ":" + r.URL.Path) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "تعداد تلاش‌ها زیاد است؛ کمی بعد دوباره امتحان کنید")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			ip := clientIP(r)
			if !s.apiLimiter.allow(ip + ":api") {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "تعداد درخواست‌ها زیاد است؛ کمی بعد دوباره امتحان کنید")
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !s.mutationLimiter.allow(ip+":mutation") {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "تعداد تغییرات زیاد است؛ کمی بعد دوباره امتحان کنید")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
