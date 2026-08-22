package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
)

var reportingRoles = []models.StaffRole{models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations}

func (s *Server) registerReportingRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/staff/reports/operations", s.require(auth.ActorStaff, reportingRoles...)(http.HandlerFunc(s.operationalReport)))
	mux.Handle("GET /api/v1/staff/audit-logs", s.require(auth.ActorStaff, models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin)(http.HandlerFunc(s.auditLogs)))
}

func (s *Server) requireReporting(w http.ResponseWriter) bool {
	if s.reporting == nil {
		writeError(w, http.StatusServiceUnavailable, "reporting_unavailable", "گزارش‌گیری در دسترس نیست")
		return false
	}
	return true
}

func (s *Server) operationalReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireReporting(w) {
		return
	}
	staff, _ := currentStaff(r)
	location, err := time.LoadLocation(staff.Hotel.Timezone)
	if err != nil {
		location = time.UTC
	}
	from, to, ok := parseReportRange(w, r, location, 31)
	if !ok {
		return
	}
	report, err := s.reporting.BuildOperationalReport(r.Context(), staff.HotelID, from, to, location)
	if err != nil {
		s.logger.Error("build operations report", "error", err, "hotelId", staff.HotelID)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ساخت گزارش انجام نشد")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"report": report})
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireReporting(w) {
		return
	}
	staff, _ := currentStaff(r)
	location, err := time.LoadLocation(staff.Hotel.Timezone)
	if err != nil {
		location = time.UTC
	}
	from, to, ok := parseReportRange(w, r, location, 31)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(defaultString(r.URL.Query().Get("limit"), "50"))
	if err != nil || limit < 1 || limit > 200 {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "تعداد نتایج معتبر نیست")
		return
	}
	offset, err := strconv.Atoi(defaultString(r.URL.Query().Get("offset"), "0"))
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "صفحه‌بندی معتبر نیست")
		return
	}
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if len(action) > 80 {
		writeError(w, http.StatusBadRequest, "invalid_action", "نوع رویداد معتبر نیست")
		return
	}
	outcome := models.AuditOutcome(strings.TrimSpace(r.URL.Query().Get("outcome")))
	if outcome != "" && outcome != models.AuditOutcomeSuccess && outcome != models.AuditOutcomeFailure {
		writeError(w, http.StatusBadRequest, "invalid_outcome", "نتیجه رویداد معتبر نیست")
		return
	}
	page, err := s.reporting.ListAuditLogs(r.Context(), staff.HotelID, store.AuditFilter{
		From: from, To: to, Action: action, Outcome: outcome, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.logger.Error("list audit logs", "error", err, "hotelId", staff.HotelID)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت رویدادهای امنیتی انجام نشد")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": page})
}

func parseReportRange(w http.ResponseWriter, r *http.Request, location *time.Location, maxDays int) (time.Time, time.Time, bool) {
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	fromLocal, toLocal := today, today.AddDate(0, 0, 1)
	var err error
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		fromLocal, err = time.ParseInLocation("2006-01-02", value, location)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_range", "تاریخ شروع معتبر نیست")
			return time.Time{}, time.Time{}, false
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", value, location)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_range", "تاریخ پایان معتبر نیست")
			return time.Time{}, time.Time{}, false
		}
		toLocal = parsed.AddDate(0, 0, 1)
	}
	if !toLocal.After(fromLocal) || toLocal.Sub(fromLocal) > time.Duration(maxDays)*24*time.Hour+2*time.Hour {
		writeError(w, http.StatusBadRequest, "invalid_range", "بازه گزارش باید حداکثر ۳۱ روز باشد")
		return time.Time{}, time.Time{}, false
	}
	return fromLocal.UTC(), toLocal.UTC(), true
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
