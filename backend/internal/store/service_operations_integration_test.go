package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
)

func TestServiceRequestOperationsPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close(db)
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	repository := store.New(db)
	suffix := uuid.NewString()[:8]
	hotel := models.Hotel{Name: "Operations " + suffix, Slug: "operations-" + suffix, PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
	admin := models.StaffUser{FirstName: "Admin", LastName: suffix, Email: "admin-" + suffix + "@example.com", PasswordHash: "unused-test-hash", Role: models.StaffRolePrimaryAdmin, IsActive: true}
	onboarding := store.HotelOnboarding{Hotel: hotel, PrimaryAdmin: admin}
	if err := repository.CreateHotelWithPrimaryAdmin(ctx, &onboarding); err != nil {
		t.Fatalf("create hotel with core services: %v", err)
	}
	hotel, admin = onboarding.Hotel, onboarding.PrimaryAdmin
	services, err := repository.ListGuestServices(ctx, hotel.ID)
	if err != nil || len(services) != 6 {
		t.Fatalf("expected six core services, got %d: %v", len(services), err)
	}
	var housekeepingService models.Service
	for _, service := range services {
		if service.Code == "room-cleaning" {
			housekeepingService = service
		}
	}
	if housekeepingService.ID == uuid.Nil || !housekeepingService.IsQuickAction {
		t.Fatalf("missing room-cleaning quick action: %+v", housekeepingService)
	}

	guest := models.Guest{HotelID: hotel.ID, FirstName: "Guest", LastName: suffix, IdentityType: "passport", IdentityNumberHash: "unused-test-hash"}
	room := models.Room{HotelID: hotel.ID, Number: "S-" + suffix, Floor: 3, Type: "Double", Status: models.RoomStatusOccupied}
	if err := db.Create(&guest).Error; err != nil {
		t.Fatalf("create guest: %v", err)
	}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	stay := models.Stay{HotelID: hotel.ID, GuestID: guest.ID, RoomID: room.ID, Status: models.StayActive}
	if err := db.Create(&stay).Error; err != nil {
		t.Fatalf("create active stay: %v", err)
	}
	stay.Guest, stay.Room, stay.Hotel = guest, room, hotel

	now := time.Now().UTC().Truncate(time.Millisecond)
	request, err := repository.CreateServiceRequest(ctx, stay, housekeepingService.ID, 2, "بعد از ساعت ۱۰", now)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if request.HotelID != hotel.ID || request.Status != models.RequestNew || request.Quantity != 2 || len(request.Events) != 1 {
		t.Fatalf("unexpected request: %+v", request)
	}

	housekeeper := models.StaffUser{HotelID: hotel.ID, FirstName: "House", LastName: "Keeper", Email: "house-" + suffix + "@example.com", PasswordHash: "unused-test-hash", Role: models.StaffRoleHousekeeping, IsActive: true}
	if err := db.Create(&housekeeper).Error; err != nil {
		t.Fatalf("create housekeeper: %v", err)
	}
	request, err = repository.AssignServiceRequest(ctx, hotel.ID, request.ID, admin.ID, housekeeper.ID, now.Add(time.Minute))
	if err != nil || request.AssignedToID == nil || *request.AssignedToID != housekeeper.ID {
		t.Fatalf("assign request: %+v err=%v", request, err)
	}
	request, err = repository.UpdateRequestPriority(ctx, hotel.ID, request.ID, admin.ID, 3, now.Add(2*time.Minute))
	if err != nil || request.Priority != 3 {
		t.Fatalf("prioritize request: %+v err=%v", request, err)
	}
	request, err = repository.TransitionServiceRequest(ctx, hotel.ID, request.ID, housekeeper.ID, models.RequestInProgress, "شروع شد", now.Add(3*time.Minute))
	if err != nil || request.Status != models.RequestInProgress || request.StartedAt == nil {
		t.Fatalf("start request: %+v err=%v", request, err)
	}
	request, err = repository.AddRequestNote(ctx, hotel.ID, request.ID, housekeeper.ID, "مهمان پاسخ داد", now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("add note: %v", err)
	}
	request, err = repository.TransitionServiceRequest(ctx, hotel.ID, request.ID, housekeeper.ID, models.RequestCompleted, "تحویل شد", now.Add(5*time.Minute))
	if err != nil || request.Status != models.RequestCompleted || request.CompletedAt == nil || len(request.Events) != 6 {
		t.Fatalf("complete request with history: %+v err=%v", request, err)
	}
	if _, err := repository.CancelGuestRequest(ctx, hotel.ID, stay.ID, guest.ID, request.ID, "cancel", now); !errors.Is(err, store.ErrInvalidTransition) {
		t.Fatalf("completed request cancellation must fail, got %v", err)
	}

	queue, err := repository.ListStaffRequests(ctx, hotel.ID, store.RequestFilter{Category: models.ServiceCategoryHousekeeping})
	if err != nil || len(queue) != 1 || queue[0].ID != request.ID {
		t.Fatalf("department queue mismatch: %+v err=%v", queue, err)
	}
	otherHotel := models.Hotel{Name: "Other " + suffix, Slug: "other-operations-" + suffix, PrimaryColor: "#0f766e", Timezone: "UTC"}
	if err := db.Create(&otherHotel).Error; err != nil {
		t.Fatalf("create other hotel: %v", err)
	}
	otherQueue, err := repository.ListStaffRequests(ctx, otherHotel.ID, store.RequestFilter{})
	if err != nil || len(otherQueue) != 0 {
		t.Fatalf("request leaked across tenants: %+v err=%v", otherQueue, err)
	}
}
