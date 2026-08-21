package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestReservationStayLifecyclePostgres(t *testing.T) {
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

	suffix := uuid.NewString()[:8]
	hotel := models.Hotel{Name: "Lifecycle " + suffix, Slug: "lifecycle-" + suffix, PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
	if err := db.Create(&hotel).Error; err != nil {
		t.Fatalf("create hotel: %v", err)
	}
	repository := store.New(db)
	room := models.Room{HotelID: hotel.ID, Number: "T-" + suffix, Floor: 2, Type: "Double", Status: models.RoomStatusAvailable}
	if err := repository.CreateRoom(ctx, &room); err != nil {
		t.Fatalf("create room: %v", err)
	}

	identityHash, _ := auth.HashIdentity("TEST-123456")
	guest := models.Guest{FirstName: "Test", LastName: "Guest", IdentityType: "passport", IdentityNumberHash: identityHash}
	arrival := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	departure := arrival.Add(48 * time.Hour)
	reservation := models.Reservation{HotelID: hotel.ID, RoomID: &room.ID, ConfirmationCode: "T" + suffix, Status: models.ReservationPending, ArrivalDate: arrival, DepartureDate: departure}
	if err := repository.CreateReservation(ctx, &guest, &reservation); err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	overlapGuest := models.Guest{FirstName: "Other", LastName: "Guest", IdentityType: "passport", IdentityNumberHash: identityHash}
	overlap := models.Reservation{HotelID: hotel.ID, RoomID: &room.ID, ConfirmationCode: "O" + suffix, Status: models.ReservationPending, ArrivalDate: arrival.Add(time.Hour), DepartureDate: departure.Add(time.Hour)}
	if err := repository.CreateReservation(ctx, &overlapGuest, &overlap); !errors.Is(err, store.ErrReservationOverlap) {
		t.Fatalf("expected overlap error, got %v", err)
	}

	confirmed, stay, err := repository.ConfirmReservation(ctx, hotel.ID, reservation.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("confirm reservation: %v", err)
	}
	if confirmed.Status != models.ReservationConfirmed || stay.Status != models.StayPreArrival || stay.ReservationID == nil {
		t.Fatalf("unexpected confirmed lifecycle: reservation=%+v stay=%+v", confirmed, stay)
	}
	_, idempotentStay, err := repository.ConfirmReservation(ctx, hotel.ID, reservation.ID, time.Now().UTC())
	if err != nil || idempotentStay.ID != stay.ID {
		t.Fatalf("confirmation must be idempotent: stay=%s err=%v", idempotentStay.ID, err)
	}

	active, err := repository.CheckInStay(ctx, hotel.ID, stay.ID, room.ID, time.Now().UTC())
	if err != nil || active.Status != models.StayActive || active.CheckInAt == nil {
		t.Fatalf("check in: stay=%+v err=%v", active, err)
	}
	var occupied models.Room
	if err := db.First(&occupied, "id = ?", room.ID).Error; err != nil || occupied.Status != models.RoomStatusOccupied {
		t.Fatalf("room not occupied: %+v err=%v", occupied, err)
	}
	if _, err := repository.CheckInStay(ctx, hotel.ID, stay.ID, room.ID, time.Now().UTC()); !errors.Is(err, store.ErrInvalidTransition) {
		t.Fatalf("expected repeated check-in rejection, got %v", err)
	}

	checkedOut, err := repository.CheckOutStay(ctx, hotel.ID, stay.ID, time.Now().UTC())
	if err != nil || checkedOut.Status != models.StayCheckedOut || checkedOut.CheckOutAt == nil {
		t.Fatalf("check out: stay=%+v err=%v", checkedOut, err)
	}
	var cleaning models.Room
	if err := db.First(&cleaning, "id = ?", room.ID).Error; err != nil || cleaning.Status != models.RoomStatusCleaning {
		t.Fatalf("room not cleaning: %+v err=%v", cleaning, err)
	}
	var completed models.Reservation
	if err := db.First(&completed, "id = ?", reservation.ID).Error; err != nil || completed.Status != models.ReservationCompleted {
		t.Fatalf("reservation not completed: %+v err=%v", completed, err)
	}
	available, err := repository.UpdateRoomStatus(ctx, hotel.ID, room.ID, models.RoomStatusAvailable)
	if err != nil || available.Status != models.RoomStatusAvailable {
		t.Fatalf("release room after cleaning: room=%+v err=%v", available, err)
	}

	if _, err := repository.FindReservationForGuestLogin(ctx, hotel.Slug, reservation.ConfirmationCode); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("completed reservation must not permit pre-arrival login, got %v", err)
	}
}
