package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeStore struct {
	hotel                models.Hotel
	staff                models.StaffUser
	stay                 models.Stay
	createdStaff         *models.StaffUser
	audits               []models.AuditLog
	lastStaffLookupHotel uuid.UUID
}

func (f *fakeStore) CreateHotelWithPrimaryAdmin(_ context.Context, input *store.HotelOnboarding) error {
	if f.hotel.ID != uuid.Nil {
		return gorm.ErrDuplicatedKey
	}
	input.Hotel.ID = uuid.New()
	input.PrimaryAdmin.ID = uuid.New()
	input.PrimaryAdmin.HotelID = input.Hotel.ID
	input.PrimaryAdmin.Hotel = input.Hotel
	f.hotel, f.staff = input.Hotel, input.PrimaryAdmin
	return nil
}

func (f *fakeStore) FindHotelBySlug(_ context.Context, slug string) (models.Hotel, error) {
	if f.hotel.Slug != slug {
		return models.Hotel{}, gorm.ErrRecordNotFound
	}
	return f.hotel, nil
}

func (f *fakeStore) FindStaffForLogin(_ context.Context, slug, email string) (models.StaffUser, error) {
	if f.staff.Hotel.Slug != slug || f.staff.Email != email {
		return models.StaffUser{}, gorm.ErrRecordNotFound
	}
	return f.staff, nil
}

func (f *fakeStore) FindStaffByID(_ context.Context, hotelID, staffID uuid.UUID) (models.StaffUser, error) {
	f.lastStaffLookupHotel = hotelID
	if f.staff.HotelID != hotelID || f.staff.ID != staffID {
		return models.StaffUser{}, gorm.ErrRecordNotFound
	}
	return f.staff, nil
}

func (f *fakeStore) MarkStaffLogin(_ context.Context, hotelID, staffID uuid.UUID, at time.Time) error {
	if f.staff.HotelID != hotelID || f.staff.ID != staffID {
		return gorm.ErrRecordNotFound
	}
	f.staff.LastLoginAt = &at
	return nil
}

func (f *fakeStore) CreateStaff(_ context.Context, staff *models.StaffUser) error {
	staff.ID = uuid.New()
	f.createdStaff = staff
	return nil
}

func (f *fakeStore) ListStaff(_ context.Context, hotelID uuid.UUID) ([]models.StaffUser, error) {
	if f.staff.HotelID != hotelID {
		return nil, gorm.ErrRecordNotFound
	}
	return []models.StaffUser{f.staff}, nil
}

func (f *fakeStore) UpdateHotelBranding(_ context.Context, hotelID uuid.UUID, name, logoURL, color, timezone string) (models.Hotel, error) {
	if f.hotel.ID != hotelID {
		return models.Hotel{}, gorm.ErrRecordNotFound
	}
	f.hotel.Name, f.hotel.LogoURL, f.hotel.PrimaryColor, f.hotel.Timezone = name, logoURL, color, timezone
	return f.hotel, nil
}

func (f *fakeStore) FindActiveStayForGuestLogin(_ context.Context, slug, room string) (models.Stay, error) {
	if f.stay.Hotel.Slug != slug || f.stay.Room.Number != room {
		return models.Stay{}, gorm.ErrRecordNotFound
	}
	return f.stay, nil
}

func (f *fakeStore) FindGuestSession(_ context.Context, hotelID, guestID, stayID uuid.UUID) (models.Stay, error) {
	if f.stay.HotelID != hotelID || f.stay.GuestID != guestID || f.stay.ID != stayID || (f.stay.Status != models.StayActive && f.stay.Status != models.StayPreArrival) {
		return models.Stay{}, gorm.ErrRecordNotFound
	}
	return f.stay, nil
}

func (f *fakeStore) WriteAudit(_ context.Context, audit *models.AuditLog) error {
	f.audits = append(f.audits, *audit)
	return nil
}

func testTokens(t *testing.T) *auth.TokenManager {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-secret-that-is-at-least-thirty-two-characters", "hotelmate-test", time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("create tokens: %v", err)
	}
	return tokens
}

func testHandler(f *fakeStore, tokens *auth.TokenManager) http.Handler {
	return NewHandler(Dependencies{Store: f, Tokens: tokens, Version: "test", AllowedOrigins: []string{"*"}, OnboardingToken: "test-onboarding-token"})
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "health-check-123")
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Version: "test", AllowedOrigins: []string{"*"}}).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("missing security header: %q", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("missing content security policy")
	}
	if got := res.Header().Get("Permissions-Policy"); got == "" {
		t.Fatal("missing permissions policy")
	}
	if got := res.Header().Get("Cross-Origin-Resource-Policy"); got != "same-site" {
		t.Fatalf("missing cross-origin resource policy: %q", got)
	}
	if got := res.Header().Get("X-Request-ID"); got != "health-check-123" {
		t.Fatalf("valid request id was not preserved: %q", got)
	}
}

func TestPrometheusMetricsExposeBoundedRoutePatterns(t *testing.T) {
	handler := NewHandler(Dependencies{Version: "test", AllowedOrigins: []string{"*"}})
	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(httptest.NewRecorder(), health)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "hotelmate_build_info{version=\"test\"} 1") || !strings.Contains(body, "pattern=\"GET /healthz\"") {
		t.Fatalf("unexpected metrics body: %s", body)
	}
}

func TestInvalidRequestIDIsReplacedAndProductionHSTSIsSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "invalid request id")
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Version: "test", EnableHSTS: true}).ServeHTTP(res, req)
	if got := res.Header().Get("X-Request-ID"); !validRequestID.MatchString(got) || got == "invalid request id" {
		t.Fatalf("invalid request id was not replaced: %q", got)
	}
	if got := res.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("missing production HSTS header")
	}
}

func TestReadyHandlerWithoutDatabase(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{}).ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}
}

func TestStaffLoginAndTenantBoundSession(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Test Hotel", Slug: "test-hotel", PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
	hash, _ := auth.HashPassword("correct-horse-battery-staple")
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, FirstName: "Test", LastName: "Admin", Email: "admin@example.com", PasswordHash: hash, Role: models.StaffRolePrimaryAdmin, IsActive: true}
	fake := &fakeStore{hotel: hotel, staff: staff}
	handler := testHandler(fake, testTokens(t))

	body := []byte(`{"hotelSlug":"test-hotel","email":"admin@example.com","password":"correct-horse-battery-staple"}`)
	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/staff/login", bytes.NewReader(body))
	login.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	handler.ServeHTTP(loginRes, login)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &response); err != nil || response.Token == "" {
		t.Fatalf("decode login response: %v", err)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/staff/me", nil)
	me.Header.Set("Authorization", "Bearer "+response.Token)
	meRes := httptest.NewRecorder()
	handler.ServeHTTP(meRes, me)
	if meRes.Code != http.StatusOK {
		t.Fatalf("me status %d: %s", meRes.Code, meRes.Body.String())
	}
	if fake.lastStaffLookupHotel != hotel.ID {
		t.Fatalf("middleware did not scope lookup to token tenant: %s", fake.lastStaffLookupHotel)
	}
	if len(fake.audits) == 0 || fake.audits[0].Outcome != models.AuditOutcomeSuccess {
		t.Fatal("successful login was not audited")
	}
	if fake.audits[0].RequestID == "" || fake.audits[0].RequestID != loginRes.Header().Get("X-Request-ID") {
		t.Fatalf("audit request id mismatch: %+v", fake.audits[0])
	}
}

func TestGuestTokenCannotAccessStaffEndpoint(t *testing.T) {
	hotelID, guestID, stayID := uuid.New(), uuid.New(), uuid.New()
	tokens := testTokens(t)
	token, _, _ := tokens.IssueGuest(models.Stay{BaseModel: models.BaseModel{ID: stayID}, HotelID: hotelID, GuestID: guestID})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/staff/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	testHandler(&fakeStore{}, tokens).ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected guest token rejection, got %d", res.Code)
	}
}

func TestGuestLoginAndActiveStaySession(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Guest Hotel", Slug: "guest-hotel", PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
	identityHash, _ := auth.HashIdentity("AB-123456")
	guest := models.Guest{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, FirstName: "Demo", LastName: "Guest", IdentityNumberHash: identityHash}
	room := models.Room{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Number: "101", Floor: 1, Type: "Double"}
	stay := models.Stay{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, GuestID: guest.ID, Guest: guest, RoomID: room.ID, Room: room, Status: models.StayActive}
	fake := &fakeStore{hotel: hotel, stay: stay}
	tokens := testTokens(t)
	handler := testHandler(fake, tokens)

	body := []byte(`{"hotelSlug":"guest-hotel","roomNumber":"101","identityNumber":"ab 123456"}`)
	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/guest/login", bytes.NewReader(body))
	loginRes := httptest.NewRecorder()
	handler.ServeHTTP(loginRes, login)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("guest login status %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var response struct {
		Token     string `json:"token"`
		ActorType string `json:"actorType"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &response); err != nil || response.Token == "" || response.ActorType != "guest" {
		t.Fatalf("decode guest login response: %v (%+v)", err, response)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/guest/me", nil)
	me.Header.Set("Authorization", "Bearer "+response.Token)
	meRes := httptest.NewRecorder()
	handler.ServeHTTP(meRes, me)
	if meRes.Code != http.StatusOK {
		t.Fatalf("guest me status %d: %s", meRes.Code, meRes.Body.String())
	}
}

func TestCreateStaffAlwaysUsesAuthenticatedTenant(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "tenant-a"}
	admin := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRolePrimaryAdmin, IsActive: true}
	fake := &fakeStore{hotel: hotel, staff: admin}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(admin)
	body := []byte(`{"firstName":"Sara","lastName":"Ahmadi","email":"sara@example.com","password":"a-strong-password","role":"reception"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	testHandler(fake, tokens).ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create staff status %d: %s", res.Code, res.Body.String())
	}
	if fake.createdStaff == nil || fake.createdStaff.HotelID != hotel.ID {
		t.Fatal("created staff was not bound to authenticated tenant")
	}
}

func TestOnboardingRequiresDeploymentToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/hotels", bytes.NewBufferString(`{}`))
	res := httptest.NewRecorder()
	testHandler(&fakeStore{}, testTokens(t)).ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}
}

var _ store.Store = (*fakeStore)(nil)
