package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeLifecycleStore struct {
	reservation       models.Reservation
	stay              models.Stay
	rooms             []models.Room
	onlineCheckIn     models.OnlineCheckIn
	lastHotelID       uuid.UUID
	lastReviewHotelID uuid.UUID
}

func (f *fakeLifecycleStore) CreateRoom(_ context.Context, room *models.Room) error {
	f.lastHotelID = room.HotelID
	room.ID = uuid.New()
	f.rooms = append(f.rooms, *room)
	return nil
}

func (f *fakeLifecycleStore) ListRooms(_ context.Context, hotelID uuid.UUID) ([]models.Room, error) {
	f.lastHotelID = hotelID
	return f.rooms, nil
}

func (f *fakeLifecycleStore) UpdateRoomStatus(_ context.Context, hotelID, roomID uuid.UUID, status models.RoomStatus) (models.Room, error) {
	f.lastHotelID = hotelID
	for index := range f.rooms {
		if f.rooms[index].ID == roomID {
			f.rooms[index].Status = status
			return f.rooms[index], nil
		}
	}
	return models.Room{}, gorm.ErrRecordNotFound
}

func (f *fakeLifecycleStore) CreateReservation(_ context.Context, guest *models.Guest, reservation *models.Reservation) error {
	f.lastHotelID = reservation.HotelID
	guest.ID, guest.HotelID = uuid.New(), reservation.HotelID
	reservation.ID, reservation.GuestID, reservation.Guest = uuid.New(), guest.ID, *guest
	f.reservation = *reservation
	return nil
}

func (f *fakeLifecycleStore) ListReservations(_ context.Context, hotelID uuid.UUID, _ store.ReservationFilter) ([]models.Reservation, error) {
	f.lastHotelID = hotelID
	if f.reservation.ID == uuid.Nil {
		return []models.Reservation{}, nil
	}
	return []models.Reservation{f.reservation}, nil
}

func (f *fakeLifecycleStore) FindReservationForGuestLogin(_ context.Context, hotelSlug, code string) (models.Reservation, error) {
	if f.reservation.Hotel.Slug != hotelSlug || f.reservation.ConfirmationCode != code {
		return models.Reservation{}, gorm.ErrRecordNotFound
	}
	return f.reservation, nil
}

func (f *fakeLifecycleStore) EnsurePreArrivalStay(_ context.Context, hotelID, reservationID uuid.UUID) (models.Stay, error) {
	f.lastHotelID = hotelID
	if f.reservation.ID != reservationID || f.reservation.HotelID != hotelID {
		return models.Stay{}, gorm.ErrRecordNotFound
	}
	return f.stay, nil
}

func (f *fakeLifecycleStore) ConfirmReservation(_ context.Context, hotelID, reservationID uuid.UUID, at time.Time) (models.Reservation, models.Stay, error) {
	f.lastHotelID = hotelID
	if f.reservation.ID != reservationID {
		return models.Reservation{}, models.Stay{}, gorm.ErrRecordNotFound
	}
	f.reservation.Status, f.reservation.ConfirmedAt = models.ReservationConfirmed, &at
	return f.reservation, f.stay, nil
}

func (f *fakeLifecycleStore) CheckInStay(_ context.Context, hotelID, stayID, _ uuid.UUID, at time.Time) (models.Stay, error) {
	f.lastHotelID = hotelID
	if f.stay.ID != stayID {
		return models.Stay{}, gorm.ErrRecordNotFound
	}
	f.stay.Status, f.stay.CheckInAt = models.StayActive, &at
	return f.stay, nil
}

func (f *fakeLifecycleStore) CheckOutStay(_ context.Context, hotelID, stayID uuid.UUID, at time.Time) (models.Stay, error) {
	f.lastHotelID = hotelID
	if f.stay.ID != stayID {
		return models.Stay{}, gorm.ErrRecordNotFound
	}
	f.stay.Status, f.stay.CheckOutAt = models.StayCheckedOut, &at
	return f.stay, nil
}

func (f *fakeLifecycleStore) GetOnlineCheckInByStay(_ context.Context, hotelID, stayID uuid.UUID) (models.OnlineCheckIn, error) {
	f.lastHotelID = hotelID
	if f.onlineCheckIn.ID == uuid.Nil || f.onlineCheckIn.StayID != stayID {
		return models.OnlineCheckIn{}, gorm.ErrRecordNotFound
	}
	return f.onlineCheckIn, nil
}

func (f *fakeLifecycleStore) UpsertOnlineCheckIn(_ context.Context, hotelID, stayID uuid.UUID, document models.OnlineCheckIn) (models.OnlineCheckIn, string, error) {
	f.lastHotelID = hotelID
	oldKey := f.onlineCheckIn.DocumentStorageKey
	document.ID, document.HotelID, document.StayID = uuid.New(), hotelID, stayID
	document.Status, document.Stay = models.OnlineCheckInSubmitted, f.stay
	f.onlineCheckIn = document
	return document, oldKey, nil
}

func (f *fakeLifecycleStore) ListOnlineCheckIns(_ context.Context, hotelID uuid.UUID, _ models.OnlineCheckInStatus) ([]models.OnlineCheckIn, error) {
	f.lastHotelID = hotelID
	if f.onlineCheckIn.ID == uuid.Nil {
		return []models.OnlineCheckIn{}, nil
	}
	return []models.OnlineCheckIn{f.onlineCheckIn}, nil
}

func (f *fakeLifecycleStore) GetOnlineCheckIn(_ context.Context, hotelID, checkInID uuid.UUID) (models.OnlineCheckIn, error) {
	f.lastHotelID = hotelID
	if f.onlineCheckIn.ID != checkInID || f.onlineCheckIn.HotelID != hotelID {
		return models.OnlineCheckIn{}, gorm.ErrRecordNotFound
	}
	return f.onlineCheckIn, nil
}

func (f *fakeLifecycleStore) ReviewOnlineCheckIn(_ context.Context, hotelID, checkInID, reviewerID uuid.UUID, status models.OnlineCheckInStatus, note string, at time.Time) (models.OnlineCheckIn, error) {
	f.lastReviewHotelID = hotelID
	if f.onlineCheckIn.ID != checkInID || f.onlineCheckIn.HotelID != hotelID {
		return models.OnlineCheckIn{}, gorm.ErrRecordNotFound
	}
	f.onlineCheckIn.Status, f.onlineCheckIn.ReviewNote = status, note
	f.onlineCheckIn.ReviewedAt, f.onlineCheckIn.ReviewedByID = &at, &reviewerID
	return f.onlineCheckIn, nil
}

func (f *fakeLifecycleStore) ListExpiredDocuments(_ context.Context, _ time.Time, _ int) ([]models.OnlineCheckIn, error) {
	return nil, nil
}

func (f *fakeLifecycleStore) MarkDocumentDeleted(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

func lifecycleHandler(base *fakeStore, lifecycle *fakeLifecycleStore, tokens *auth.TokenManager, storage documents.Storage) http.Handler {
	return NewHandler(Dependencies{
		Store: base, Lifecycle: lifecycle, Documents: storage, Tokens: tokens, Version: "test",
		AllowedOrigins: []string{"*"}, DocumentMaxBytes: 1024 * 1024, DocumentRetention: 24 * time.Hour,
	})
}

func TestReservationLoginCreatesTenantBoundPreArrivalSession(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Arrival Hotel", Slug: "arrival-hotel", PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
	identityHash, _ := auth.HashIdentity("P-998877")
	guest := models.Guest{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, FirstName: "Nila", LastName: "Guest", IdentityNumberHash: identityHash}
	room := models.Room{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Number: "203", Status: models.RoomStatusAvailable}
	reservation := models.Reservation{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, GuestID: guest.ID, Guest: guest, RoomID: &room.ID, Room: &room, ConfirmationCode: "ABC12345", Status: models.ReservationConfirmed}
	stay := models.Stay{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, GuestID: guest.ID, Guest: guest, RoomID: room.ID, Room: room, ReservationID: &reservation.ID, Reservation: &reservation, Status: models.StayPreArrival}
	lifecycle := &fakeLifecycleStore{reservation: reservation, stay: stay}
	base := &fakeStore{hotel: hotel, stay: stay}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/guest/reservation", bytes.NewBufferString(`{"hotelSlug":"arrival-hotel","confirmationCode":"abc12345","identityNumber":"p 998877"}`))
	res := httptest.NewRecorder()
	lifecycleHandler(base, lifecycle, testTokens(t), nil).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("reservation login status %d: %s", res.Code, res.Body.String())
	}
	var response struct {
		Token string `json:"token"`
		Stay  struct {
			Status models.StayStatus `json:"status"`
		} `json:"stay"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil || response.Token == "" || response.Stay.Status != models.StayPreArrival {
		t.Fatalf("invalid reservation login response: %v (%+v)", err, response)
	}
	if lifecycle.lastHotelID != hotel.ID {
		t.Fatalf("pre-arrival stay was not tenant scoped: %s", lifecycle.lastHotelID)
	}
}

func TestLifecycleRoutesEnforceRoleAndTenant(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Slug: "role-hotel"}
	room := models.Room{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Number: "101", Status: models.RoomStatusAvailable}
	tokens := testTokens(t)

	housekeeper := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleHousekeeping, IsActive: true}
	houseToken, _, _ := tokens.IssueStaff(housekeeper)
	lifecycle := &fakeLifecycleStore{rooms: []models.Room{room}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/staff/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+houseToken)
	res := httptest.NewRecorder()
	lifecycleHandler(&fakeStore{hotel: hotel, staff: housekeeper}, lifecycle, tokens, nil).ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden role, got %d", res.Code)
	}
	if lifecycle.lastHotelID != uuid.Nil {
		t.Fatal("forbidden request reached lifecycle store")
	}

	reception := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleReception, IsActive: true}
	receptionToken, _, _ := tokens.IssueStaff(reception)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/staff/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+receptionToken)
	res = httptest.NewRecorder()
	lifecycleHandler(&fakeStore{hotel: hotel, staff: reception}, lifecycle, tokens, nil).ServeHTTP(res, req)
	if res.Code != http.StatusOK || lifecycle.lastHotelID != hotel.ID {
		t.Fatalf("tenant scoped lifecycle request failed: status=%d hotel=%s", res.Code, lifecycle.lastHotelID)
	}
}

func TestOnlineCheckInUploadAndStaffReview(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Secure Hotel", Slug: "secure-hotel", PrimaryColor: "#0f766e", Timezone: "UTC"}
	guest := models.Guest{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, FirstName: "Ava", LastName: "Guest"}
	room := models.Room{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Number: "301", Status: models.RoomStatusAvailable}
	stay := models.Stay{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, GuestID: guest.ID, Guest: guest, RoomID: room.ID, Room: room, Status: models.StayPreArrival}
	lifecycle := &fakeLifecycleStore{stay: stay}
	storage, err := documents.NewLocalStorage(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	tokens := testTokens(t)
	guestToken, _, _ := tokens.IssueGuest(stay)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("document", "passport.png")
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guest/online-check-in", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+guestToken)
	res := httptest.NewRecorder()
	base := &fakeStore{hotel: hotel, stay: stay}
	lifecycleHandler(base, lifecycle, tokens, storage).ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("upload status %d: %s", res.Code, res.Body.String())
	}
	if lifecycle.onlineCheckIn.DocumentStorageKey == "" || lifecycle.onlineCheckIn.DocumentSHA256 == "" || lifecycle.onlineCheckIn.HotelID != hotel.ID {
		t.Fatalf("private document metadata was not persisted: %+v", lifecycle.onlineCheckIn)
	}

	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleReception, IsActive: true}
	staffToken, _, _ := tokens.IssueStaff(staff)
	reviewBody := bytes.NewBufferString(`{"status":"approved","note":"verified"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/staff/online-check-ins/"+lifecycle.onlineCheckIn.ID.String()+"/review", reviewBody)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	res = httptest.NewRecorder()
	lifecycleHandler(&fakeStore{hotel: hotel, staff: staff, stay: stay}, lifecycle, tokens, storage).ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("review status %d: %s", res.Code, res.Body.String())
	}
	if lifecycle.onlineCheckIn.Status != models.OnlineCheckInApproved || lifecycle.lastReviewHotelID != hotel.ID {
		t.Fatalf("review was not tenant scoped: %+v", lifecycle.onlineCheckIn)
	}
}

var _ store.LifecycleStore = (*fakeLifecycleStore)(nil)
