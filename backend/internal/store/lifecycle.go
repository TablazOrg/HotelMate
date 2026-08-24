package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidTransition  = errors.New("invalid lifecycle transition")
	ErrRoomUnavailable    = errors.New("room is unavailable")
	ErrReservationOverlap = errors.New("reservation overlaps an existing booking")
)

type ReservationFilter struct {
	Status models.ReservationStatus
	From   *time.Time
	To     *time.Time
}

type LifecycleStore interface {
	CreateRoom(context.Context, *models.Room) error
	ListRooms(context.Context, uuid.UUID) ([]models.Room, error)
	UpdateRoomStatus(context.Context, uuid.UUID, uuid.UUID, models.RoomStatus) (models.Room, error)
	CreateReservation(context.Context, *models.Guest, *models.Reservation) error
	ListReservations(context.Context, uuid.UUID, ReservationFilter) ([]models.Reservation, error)
	FindReservationForGuestLogin(context.Context, string, string) (models.Reservation, error)
	EnsurePreArrivalStay(context.Context, uuid.UUID, uuid.UUID) (models.Stay, error)
	ConfirmReservation(context.Context, uuid.UUID, uuid.UUID, time.Time) (models.Reservation, models.Stay, error)
	CheckInStay(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (models.Stay, error)
	CheckOutStay(context.Context, uuid.UUID, uuid.UUID, time.Time) (models.Stay, error)
	GetOnlineCheckInByStay(context.Context, uuid.UUID, uuid.UUID) (models.OnlineCheckIn, error)
	UpsertOnlineCheckIn(context.Context, uuid.UUID, uuid.UUID, models.OnlineCheckIn) (models.OnlineCheckIn, string, error)
	ListOnlineCheckIns(context.Context, uuid.UUID, models.OnlineCheckInStatus) ([]models.OnlineCheckIn, error)
	GetOnlineCheckIn(context.Context, uuid.UUID, uuid.UUID) (models.OnlineCheckIn, error)
	ReviewOnlineCheckIn(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.OnlineCheckInStatus, string, time.Time) (models.OnlineCheckIn, error)
	ListExpiredDocuments(context.Context, time.Time, int) ([]models.OnlineCheckIn, error)
	MarkDocumentDeleted(context.Context, uuid.UUID, time.Time) error
}

func (s *GORMStore) CreateRoom(ctx context.Context, room *models.Room) error {
	return s.db.WithContext(ctx).Create(room).Error
}

func (s *GORMStore) ListRooms(ctx context.Context, hotelID uuid.UUID) ([]models.Room, error) {
	var rooms []models.Room
	err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).Order("floor ASC, number ASC").Find(&rooms).Error
	return rooms, err
}

func (s *GORMStore) UpdateRoomStatus(ctx context.Context, hotelID, roomID uuid.UUID, status models.RoomStatus) (models.Room, error) {
	var room models.Room
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, roomID).First(&room).Error; err != nil {
			return err
		}
		if room.Status == models.RoomStatusOccupied || status == models.RoomStatusOccupied {
			return ErrInvalidTransition
		}
		if status == models.RoomStatusAvailable {
			var count int64
			if err := tx.Model(&models.Stay{}).Where("hotel_id = ? AND room_id = ? AND status = ?", hotelID, roomID, models.StayActive).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrRoomUnavailable
			}
		}
		room.Status = status
		return tx.Model(&room).Update("status", status).Error
	})
	return room, err
}

func (s *GORMStore) CreateReservation(ctx context.Context, guest *models.Guest, reservation *models.Reservation) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if reservation.RoomID == nil {
			return ErrRoomUnavailable
		}
		var room models.Room
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", reservation.HotelID, *reservation.RoomID).First(&room).Error; err != nil {
			return err
		}
		if room.Status == models.RoomStatusOutOfSvc {
			return ErrRoomUnavailable
		}
		var overlap int64
		if err := tx.Model(&models.Reservation{}).
			Where("hotel_id = ? AND room_id = ? AND status IN ? AND arrival_date < ? AND departure_date > ?", reservation.HotelID, room.ID, []models.ReservationStatus{models.ReservationPending, models.ReservationConfirmed}, reservation.DepartureDate, reservation.ArrivalDate).
			Count(&overlap).Error; err != nil {
			return err
		}
		if overlap > 0 {
			return ErrReservationOverlap
		}
		guest.HotelID = reservation.HotelID
		if err := tx.Create(guest).Error; err != nil {
			return err
		}
		reservation.GuestID = guest.ID
		reservation.Guest = *guest
		reservation.Room = &room
		return tx.Create(reservation).Error
	})
}

func (s *GORMStore) ListReservations(ctx context.Context, hotelID uuid.UUID, filter ReservationFilter) ([]models.Reservation, error) {
	query := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.From != nil {
		query = query.Where("departure_date >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("arrival_date <= ?", *filter.To)
	}
	var reservations []models.Reservation
	err := query.Preload("Guest").Preload("Room").Preload("Stay.Room").Order("arrival_date ASC, created_at ASC").Find(&reservations).Error
	return reservations, err
}

func (s *GORMStore) FindReservationForGuestLogin(ctx context.Context, hotelSlug, code string) (models.Reservation, error) {
	var reservation models.Reservation
	err := s.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Joins("JOIN hotels ON hotels.id = reservations.hotel_id AND hotels.deleted_at IS NULL").
		Where("hotels.slug = ? AND reservations.confirmation_code = ? AND reservations.status = ?", hotelSlug, code, models.ReservationConfirmed).
		Preload("Guest").Preload("Hotel").Preload("Room").First(&reservation).Error
	return reservation, err
}

func (s *GORMStore) EnsurePreArrivalStay(ctx context.Context, hotelID, reservationID uuid.UUID) (models.Stay, error) {
	var stay models.Stay
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation models.Reservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, reservationID).First(&reservation).Error; err != nil {
			return err
		}
		if reservation.Status != models.ReservationConfirmed || reservation.RoomID == nil {
			return ErrInvalidTransition
		}
		err := tx.Where("hotel_id = ? AND reservation_id = ?", hotelID, reservationID).First(&stay).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		stay = models.Stay{HotelID: hotelID, GuestID: reservation.GuestID, RoomID: *reservation.RoomID, ReservationID: &reservation.ID, Status: models.StayPreArrival}
		return tx.Create(&stay).Error
	})
	if err != nil {
		return models.Stay{}, err
	}
	return s.loadStay(ctx, hotelID, stay.ID)
}

func (s *GORMStore) ConfirmReservation(ctx context.Context, hotelID, reservationID uuid.UUID, at time.Time) (models.Reservation, models.Stay, error) {
	var reservation models.Reservation
	var stay models.Stay
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, reservationID).First(&reservation).Error; err != nil {
			return err
		}
		if reservation.RoomID == nil {
			return ErrRoomUnavailable
		}
		if reservation.Status != models.ReservationPending && reservation.Status != models.ReservationConfirmed {
			return ErrInvalidTransition
		}
		if reservation.Status == models.ReservationPending {
			reservation.Status = models.ReservationConfirmed
			reservation.ConfirmedAt = &at
			if err := tx.Model(&reservation).Updates(map[string]any{"status": reservation.Status, "confirmed_at": at}).Error; err != nil {
				return err
			}
		}
		err := tx.Where("hotel_id = ? AND reservation_id = ?", hotelID, reservationID).First(&stay).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		stay = models.Stay{HotelID: hotelID, GuestID: reservation.GuestID, RoomID: *reservation.RoomID, ReservationID: &reservation.ID, Status: models.StayPreArrival}
		return tx.Create(&stay).Error
	})
	if err != nil {
		return models.Reservation{}, models.Stay{}, err
	}
	if err := s.db.WithContext(ctx).Preload("Guest").Preload("Room").Preload("Stay.Room").First(&reservation, "hotel_id = ? AND id = ?", hotelID, reservationID).Error; err != nil {
		return models.Reservation{}, models.Stay{}, err
	}
	loadedStay, err := s.loadStay(ctx, hotelID, stay.ID)
	return reservation, loadedStay, err
}

func (s *GORMStore) CheckInStay(ctx context.Context, hotelID, stayID, roomID uuid.UUID, at time.Time) (models.Stay, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stay models.Stay
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, stayID).First(&stay).Error; err != nil {
			return err
		}
		if stay.Status != models.StayPreArrival || stay.RoomID != roomID {
			return ErrInvalidTransition
		}
		var room models.Room
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, roomID).First(&room).Error; err != nil {
			return err
		}
		if room.Status != models.RoomStatusAvailable {
			return ErrRoomUnavailable
		}
		if stay.ReservationID != nil {
			var reservation models.Reservation
			if err := tx.Where("hotel_id = ? AND id = ? AND status = ?", hotelID, *stay.ReservationID, models.ReservationConfirmed).First(&reservation).Error; err != nil {
				return ErrInvalidTransition
			}
		}
		if err := tx.Model(&stay).Updates(map[string]any{"status": models.StayActive, "check_in_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Model(&room).Update("status", models.RoomStatusOccupied).Error; err != nil {
			return err
		}
		var journey models.ArrivalJourney
		err := tx.Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&journey).Updates(map[string]any{"status": models.ArrivalCheckedIn, "checked_in_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{
			BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID,
			EventType: "physical_arrival", ActorType: "staff",
		}).Error
	})
	if err != nil {
		return models.Stay{}, err
	}
	return s.loadStay(ctx, hotelID, stayID)
}

func (s *GORMStore) CheckOutStay(ctx context.Context, hotelID, stayID uuid.UUID, at time.Time) (models.Stay, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stay models.Stay
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, stayID).First(&stay).Error; err != nil {
			return err
		}
		if stay.Status != models.StayActive {
			return ErrInvalidTransition
		}
		var room models.Room
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, stay.RoomID).First(&room).Error; err != nil {
			return err
		}
		if err := tx.Model(&stay).Updates(map[string]any{"status": models.StayCheckedOut, "check_out_at": at}).Error; err != nil {
			return err
		}
		if err := tx.Model(&room).Update("status", models.RoomStatusCleaning).Error; err != nil {
			return err
		}
		if stay.ReservationID != nil {
			if err := tx.Model(&models.Reservation{}).Where("hotel_id = ? AND id = ?", hotelID, *stay.ReservationID).Update("status", models.ReservationCompleted).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.Stay{}, err
	}
	return s.loadStay(ctx, hotelID, stayID)
}

func (s *GORMStore) loadStay(ctx context.Context, hotelID, stayID uuid.UUID) (models.Stay, error) {
	var stay models.Stay
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, stayID).
		Preload("Hotel").Preload("Guest").Preload("Room").
		Preload("Reservation.Guest").Preload("Reservation.Room").First(&stay).Error
	return stay, err
}

func (s *GORMStore) GetOnlineCheckInByStay(ctx context.Context, hotelID, stayID uuid.UUID) (models.OnlineCheckIn, error) {
	var checkIn models.OnlineCheckIn
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&checkIn).Error
	return checkIn, err
}

func (s *GORMStore) UpsertOnlineCheckIn(ctx context.Context, hotelID, stayID uuid.UUID, document models.OnlineCheckIn) (models.OnlineCheckIn, string, error) {
	var checkIn models.OnlineCheckIn
	var oldKey string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stay models.Stay
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, stayID).First(&stay).Error; err != nil {
			return err
		}
		if stay.Status != models.StayPreArrival {
			return ErrInvalidTransition
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&checkIn).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			document.HotelID = hotelID
			document.StayID = stayID
			document.Status = models.OnlineCheckInSubmitted
			checkIn = document
			return tx.Create(&checkIn).Error
		}
		if err != nil {
			return err
		}
		oldKey = checkIn.DocumentStorageKey
		updates := map[string]any{
			"status": models.OnlineCheckInSubmitted, "document_storage_key": document.DocumentStorageKey,
			"document_name": document.DocumentName, "document_media_type": document.DocumentMediaType,
			"document_size": document.DocumentSize, "document_sha256": document.DocumentSHA256,
			"submitted_at": document.SubmittedAt, "reviewed_at": nil, "reviewed_by_id": nil,
			"review_note": "", "retention_until": document.RetentionUntil, "document_deleted_at": nil,
		}
		if err := tx.Model(&checkIn).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", checkIn.ID).First(&checkIn).Error
	})
	return checkIn, oldKey, err
}

func (s *GORMStore) ListOnlineCheckIns(ctx context.Context, hotelID uuid.UUID, status models.OnlineCheckInStatus) ([]models.OnlineCheckIn, error) {
	query := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var checkIns []models.OnlineCheckIn
	err := query.Preload("Stay.Guest").Preload("Stay.Room").Preload("Stay.Hotel").
		Preload("Stay.Reservation.Guest").Preload("Stay.Reservation.Room").
		Order("submitted_at ASC").Find(&checkIns).Error
	return checkIns, err
}

func (s *GORMStore) GetOnlineCheckIn(ctx context.Context, hotelID, checkInID uuid.UUID) (models.OnlineCheckIn, error) {
	var checkIn models.OnlineCheckIn
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, checkInID).
		Preload("Stay.Guest").Preload("Stay.Room").First(&checkIn).Error
	return checkIn, err
}

func (s *GORMStore) ReviewOnlineCheckIn(ctx context.Context, hotelID, checkInID, reviewerID uuid.UUID, status models.OnlineCheckInStatus, note string, at time.Time) (models.OnlineCheckIn, error) {
	var checkIn models.OnlineCheckIn
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, checkInID).First(&checkIn).Error; err != nil {
			return err
		}
		if checkIn.Status != models.OnlineCheckInSubmitted || (status != models.OnlineCheckInApproved && status != models.OnlineCheckInRejected) {
			return ErrInvalidTransition
		}
		return tx.Model(&checkIn).Updates(map[string]any{"status": status, "review_note": note, "reviewed_at": at, "reviewed_by_id": reviewerID}).Error
	})
	if err != nil {
		return models.OnlineCheckIn{}, err
	}
	return s.GetOnlineCheckIn(ctx, hotelID, checkInID)
}

func (s *GORMStore) ListExpiredDocuments(ctx context.Context, before time.Time, limit int) ([]models.OnlineCheckIn, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var checkIns []models.OnlineCheckIn
	err := s.db.WithContext(ctx).Where("retention_until <= ? AND document_deleted_at IS NULL", before).Order("retention_until ASC").Limit(limit).Find(&checkIns).Error
	return checkIns, err
}

func (s *GORMStore) MarkDocumentDeleted(ctx context.Context, checkInID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&models.OnlineCheckIn{}).Where("id = ? AND document_deleted_at IS NULL", checkInID).Update("document_deleted_at", at)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("mark online check-in document deleted: %w", gorm.ErrRecordNotFound)
	}
	return nil
}

var _ LifecycleStore = (*GORMStore)(nil)
