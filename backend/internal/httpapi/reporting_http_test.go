package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
)

type fakeReportingStore struct {
	report      store.OperationalReport
	audit       store.AuditPage
	lastHotelID uuid.UUID
	lastFilter  store.AuditFilter
}

func (f *fakeReportingStore) BuildOperationalReport(_ context.Context, hotelID uuid.UUID, from, to time.Time, location *time.Location) (store.OperationalReport, error) {
	f.lastHotelID = hotelID
	f.report.From, f.report.To, f.report.Timezone = from, to, location.String()
	return f.report, nil
}

func (f *fakeReportingStore) ListAuditLogs(_ context.Context, hotelID uuid.UUID, filter store.AuditFilter) (store.AuditPage, error) {
	f.lastHotelID, f.lastFilter = hotelID, filter
	return f.audit, nil
}

func TestOperationalReportUsesAuthenticatedTenant(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "report-hotel", Timezone: "Asia/Tehran"}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleOperations, IsActive: true}
	reporting := &fakeReportingStore{report: store.OperationalReport{Currency: "IRR", Summary: store.ReportSummary{OpenRequests: 3}}}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(staff)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/staff/reports/operations?from=2026-08-01&to=2026-08-07", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: staff}, Reporting: reporting, Tokens: tokens}).ServeHTTP(res, req)
	if res.Code != http.StatusOK || reporting.lastHotelID != hotel.ID {
		t.Fatalf("report status=%d tenant=%s body=%s", res.Code, reporting.lastHotelID, res.Body.String())
	}
}

func TestAuditLogIsAdminOnlyAndFiltered(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "audit-hotel", Timezone: "UTC"}
	admin := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRolePrimaryAdmin, IsActive: true}
	reporting := &fakeReportingStore{audit: store.AuditPage{Items: []models.AuditLog{}, Limit: 25}}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(admin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/staff/audit-logs?from=2026-08-01&to=2026-08-07&outcome=failure&action=staff.login&limit=25", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: admin}, Reporting: reporting, Tokens: tokens}).ServeHTTP(res, req)
	if res.Code != http.StatusOK || reporting.lastHotelID != hotel.ID || reporting.lastFilter.Outcome != models.AuditOutcomeFailure || reporting.lastFilter.Action != "staff.login" || reporting.lastFilter.Limit != 25 {
		t.Fatalf("audit status=%d tenant=%s filter=%+v body=%s", res.Code, reporting.lastHotelID, reporting.lastFilter, res.Body.String())
	}

	reception := admin
	reception.ID, reception.Role = uuid.New(), models.StaffRoleReception
	receptionToken, _, _ := tokens.IssueStaff(reception)
	forbidden := httptest.NewRequest(http.MethodGet, "/api/v1/staff/audit-logs", nil)
	forbidden.Header.Set("Authorization", "Bearer "+receptionToken)
	forbiddenRes := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: reception}, Reporting: reporting, Tokens: tokens}).ServeHTTP(forbiddenRes, forbidden)
	if forbiddenRes.Code != http.StatusForbidden {
		t.Fatalf("reception audit access status=%d", forbiddenRes.Code)
	}
}

var _ store.ReportingStore = (*fakeReportingStore)(nil)
