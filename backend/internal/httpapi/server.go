package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB                *gorm.DB
	Store             store.Store
	Lifecycle         store.LifecycleStore
	ServiceOperations store.ServiceOperationsStore
	Documents         documents.Storage
	Realtime          *realtime.Hub
	Tokens            *auth.TokenManager
	Version           string
	AllowedOrigins    []string
	OnboardingToken   string
	Logger            *slog.Logger
	DocumentMaxBytes  int64
	DocumentRetention time.Duration
}

type Server struct {
	db                *gorm.DB
	store             store.Store
	lifecycle         store.LifecycleStore
	serviceOperations store.ServiceOperationsStore
	documents         documents.Storage
	realtime          *realtime.Hub
	tokens            *auth.TokenManager
	version           string
	onboardingToken   string
	logger            *slog.Logger
	dummyHash         string
	authLimiter       *ipRateLimiter
	onboardLimiter    *ipRateLimiter
	documentMaxBytes  int64
	documentRetention time.Duration
	allowedOrigins    []string
}

func NewHandler(deps Dependencies) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	dummyHash, _ := auth.HashPassword("hotelmate-dummy-password")
	s := &Server{
		db:                deps.DB,
		store:             deps.Store,
		lifecycle:         deps.Lifecycle,
		documents:         deps.Documents,
		serviceOperations: deps.ServiceOperations,
		realtime:          deps.Realtime,
		tokens:            deps.Tokens,
		version:           deps.Version,
		onboardingToken:   deps.OnboardingToken,
		logger:            logger,
		dummyHash:         dummyHash,
		authLimiter:       newIPRateLimiter(20, 5*time.Minute),
		onboardLimiter:    newIPRateLimiter(5, 10*time.Minute),
		documentMaxBytes:  deps.DocumentMaxBytes,
		documentRetention: deps.DocumentRetention,
		allowedOrigins:    deps.AllowedOrigins,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", s.readyHandler)
	mux.HandleFunc("GET /api/v1", s.infoHandler)
	mux.HandleFunc("GET /api/v1/public/hotels/{slug}", s.publicHotel)
	mux.Handle("POST /api/v1/onboarding/hotels", s.limit(s.onboardLimiter, http.HandlerFunc(s.onboardHotel)))
	mux.Handle("POST /api/v1/auth/staff/login", s.limit(s.authLimiter, http.HandlerFunc(s.staffLogin)))
	mux.Handle("POST /api/v1/auth/guest/login", s.limit(s.authLimiter, http.HandlerFunc(s.guestLogin)))
	mux.Handle("POST /api/v1/auth/guest/reservation", s.limit(s.authLimiter, http.HandlerFunc(s.reservationLogin)))
	mux.Handle("GET /api/v1/staff/me", s.require(auth.ActorStaff)(http.HandlerFunc(s.staffMe)))
	mux.Handle("GET /api/v1/guest/me", s.require(auth.ActorGuest)(http.HandlerFunc(s.guestMe)))
	mux.Handle("GET /api/v1/staff/users", s.require(auth.ActorStaff, models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin)(http.HandlerFunc(s.listStaff)))
	mux.Handle("POST /api/v1/staff/users", s.require(auth.ActorStaff, models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin)(http.HandlerFunc(s.createStaff)))
	mux.Handle("PATCH /api/v1/staff/hotel", s.require(auth.ActorStaff, models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin)(http.HandlerFunc(s.updateHotelBranding)))
	s.registerLifecycleRoutes(mux)
	s.registerServiceOperationRoutes(mux)
	s.registerRealtimeRoute(mux)

	return withRecovery(logger, withRequestLog(logger, withSecurityHeaders(withCORS(deps.AllowedOrigins, mux))))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database_unavailable"})
		return
	}
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) infoHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "HotelMate API", "version": s.version})
}

func (s *Server) validOnboardingToken(candidate string) bool {
	if s.onboardingToken == "" || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.onboardingToken)) == 1
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "بدنه درخواست معتبر نیست")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "فقط یک شیء JSON ارسال کنید")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func withCORS(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed["*"]; ok {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Onboarding-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func withRequestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", writer.status, "duration", time.Since(started).String())
	})
}

func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal_server_error", "خطای داخلی سرور")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
