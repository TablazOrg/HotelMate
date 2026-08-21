package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
)

type fakeServiceOperations struct {
	service           models.Service
	request           models.ServiceRequest
	lastHotelID       uuid.UUID
	lastStayID        uuid.UUID
	lastFilter        store.RequestFilter
	transitionReached bool
}

func (f *fakeServiceOperations) ListGuestServices(_ context.Context, hotelID uuid.UUID) ([]models.Service, error) {
	f.lastHotelID = hotelID
	return []models.Service{f.service}, nil
}

func (f *fakeServiceOperations) ListStaffServices(_ context.Context, hotelID uuid.UUID) ([]models.Service, error) {
	f.lastHotelID = hotelID
	return []models.Service{f.service}, nil
}

func (f *fakeServiceOperations) CreateService(_ context.Context, service *models.Service) error {
	f.lastHotelID = service.HotelID
	service.ID = uuid.New()
	f.service = *service
	return nil
}

func (f *fakeServiceOperations) UpdateService(_ context.Context, hotelID, _ uuid.UUID, service models.Service) (models.Service, error) {
	f.lastHotelID = hotelID
	return service, nil
}

func (f *fakeServiceOperations) CreateServiceRequest(_ context.Context, stay models.Stay, _ uuid.UUID, quantity int, notes string, at time.Time) (models.ServiceRequest, error) {
	f.lastHotelID, f.lastStayID = stay.HotelID, stay.ID
	f.request.ID, f.request.HotelID, f.request.StayID = uuid.New(), stay.HotelID, stay.ID
	f.request.Stay, f.request.Service, f.request.Quantity, f.request.Notes = stay, f.service, quantity, notes
	f.request.Status, f.request.CreatedAt = models.RequestNew, at
	return f.request, nil
}

func (f *fakeServiceOperations) ListGuestRequests(_ context.Context, hotelID, stayID uuid.UUID) ([]models.ServiceRequest, error) {
	f.lastHotelID, f.lastStayID = hotelID, stayID
	return []models.ServiceRequest{f.request}, nil
}

func (f *fakeServiceOperations) ListStaffRequests(_ context.Context, hotelID uuid.UUID, filter store.RequestFilter) ([]models.ServiceRequest, error) {
	f.lastHotelID, f.lastFilter = hotelID, filter
	return []models.ServiceRequest{}, nil
}

func (f *fakeServiceOperations) GetServiceRequest(_ context.Context, hotelID, _ uuid.UUID) (models.ServiceRequest, error) {
	f.lastHotelID = hotelID
	return f.request, nil
}

func (f *fakeServiceOperations) AssignServiceRequest(_ context.Context, hotelID, _ uuid.UUID, _, _ uuid.UUID, _ time.Time) (models.ServiceRequest, error) {
	f.lastHotelID = hotelID
	return f.request, nil
}

func (f *fakeServiceOperations) UpdateRequestPriority(_ context.Context, hotelID, _ uuid.UUID, _ uuid.UUID, priority int, _ time.Time) (models.ServiceRequest, error) {
	f.lastHotelID, f.request.Priority = hotelID, priority
	return f.request, nil
}

func (f *fakeServiceOperations) TransitionServiceRequest(_ context.Context, hotelID, _ uuid.UUID, _ uuid.UUID, status models.RequestStatus, _ string, _ time.Time) (models.ServiceRequest, error) {
	f.lastHotelID, f.transitionReached, f.request.Status = hotelID, true, status
	return f.request, nil
}

func (f *fakeServiceOperations) AddRequestNote(_ context.Context, hotelID, _ uuid.UUID, _ uuid.UUID, _ string, _ time.Time) (models.ServiceRequest, error) {
	f.lastHotelID = hotelID
	return f.request, nil
}

func (f *fakeServiceOperations) CancelGuestRequest(_ context.Context, hotelID, stayID, _ uuid.UUID, _ uuid.UUID, _ string, _ time.Time) (models.ServiceRequest, error) {
	f.lastHotelID, f.lastStayID = hotelID, stayID
	f.request.Status = models.RequestCancelled
	return f.request, nil
}

func TestGuestRequestCreationIsStayScopedAndPublished(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Service Hotel", Slug: "service-hotel"}
	guest := models.Guest{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, FirstName: "Guest", LastName: "Test"}
	room := models.Room{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Number: "501", Status: models.RoomStatusOccupied}
	stay := models.Stay{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, GuestID: guest.ID, Guest: guest, RoomID: room.ID, Room: room, Status: models.StayActive}
	service := models.Service{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Code: "room-cleaning", Name: "نظافت", Category: models.ServiceCategoryHousekeeping, FulfillmentRole: models.StaffRoleHousekeeping, Currency: "IRR", IsActive: true}
	operations := &fakeServiceOperations{service: service}
	hub := realtime.NewHub()
	events, unsubscribe := hub.Subscribe(realtime.Subscriber{ActorType: auth.ActorGuest, HotelID: hotel.ID, StayID: stay.ID})
	defer unsubscribe()
	tokens := testTokens(t)
	token, _, _ := tokens.IssueGuest(stay)
	handler := NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, stay: stay}, ServiceOperations: operations, Realtime: hub, Tokens: tokens, AllowedOrigins: []string{"*"}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/guest/requests", bytes.NewBufferString(`{"serviceId":"`+service.ID.String()+`","quantity":1,"notes":"بعد از ظهر"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create request status %d: %s", res.Code, res.Body.String())
	}
	if operations.lastHotelID != hotel.ID || operations.lastStayID != stay.ID {
		t.Fatalf("request was not stay scoped: hotel=%s stay=%s", operations.lastHotelID, operations.lastStayID)
	}
	select {
	case event := <-events:
		if event.Type != "request.created" || event.StayID != stay.ID {
			t.Fatalf("unexpected realtime event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("request event was not published")
	}
}

func TestDepartmentRoleForcesQueueFulfillmentRole(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "department-hotel"}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleHousekeeping, IsActive: true}
	operations := &fakeServiceOperations{}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(staff)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/staff/requests?category=food_beverage", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: staff}, ServiceOperations: operations, Tokens: tokens, AllowedOrigins: []string{"*"}}).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("queue status %d: %s", res.Code, res.Body.String())
	}
	if operations.lastFilter.FulfillmentRole != models.StaffRoleHousekeeping || operations.lastFilter.Category != models.ServiceCategoryFNB || operations.lastHotelID != hotel.ID {
		t.Fatalf("fulfillment role was not enforced: %+v", operations.lastFilter)
	}
}

func TestDepartmentCannotMutateAnotherDepartmentsRequest(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "isolated-department"}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleHousekeeping, IsActive: true}
	operations := &fakeServiceOperations{request: models.ServiceRequest{
		BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Status: models.RequestNew,
		Service: models.Service{Category: models.ServiceCategoryHousekeeping, FulfillmentRole: models.StaffRoleFB},
	}}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(staff)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/requests/"+operations.request.ID.String()+"/transition", bytes.NewBufferString(`{"status":"in_progress","note":""}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: staff}, ServiceOperations: operations, Tokens: tokens, AllowedOrigins: []string{"*"}}).ServeHTTP(res, req)
	if res.Code != http.StatusForbidden || operations.transitionReached {
		t.Fatalf("cross-department mutation was not rejected: status=%d reached=%v", res.Code, operations.transitionReached)
	}
}

var _ store.ServiceOperationsStore = (*fakeServiceOperations)(nil)
