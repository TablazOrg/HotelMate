package store

import (
	"context"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HotelOnboarding struct {
	Hotel        models.Hotel
	PrimaryAdmin models.StaffUser
}

type Store interface {
	CreateHotelWithPrimaryAdmin(context.Context, *HotelOnboarding) error
	FindHotelBySlug(context.Context, string) (models.Hotel, error)
	FindStaffForLogin(context.Context, string, string) (models.StaffUser, error)
	FindStaffByID(context.Context, uuid.UUID, uuid.UUID) (models.StaffUser, error)
	MarkStaffLogin(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	CreateStaff(context.Context, *models.StaffUser) error
	ListStaff(context.Context, uuid.UUID) ([]models.StaffUser, error)
	UpdateHotelBranding(context.Context, uuid.UUID, string, string, string, string) (models.Hotel, error)
	FindActiveStayForGuestLogin(context.Context, string, string) (models.Stay, error)
	FindGuestSession(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (models.Stay, error)
	WriteAudit(context.Context, *models.AuditLog) error
}

type GORMStore struct {
	db *gorm.DB
}

func New(db *gorm.DB) *GORMStore { return &GORMStore{db: db} }

func (s *GORMStore) CreateHotelWithPrimaryAdmin(ctx context.Context, onboarding *HotelOnboarding) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&onboarding.Hotel).Error; err != nil {
			return err
		}
		onboarding.PrimaryAdmin.HotelID = onboarding.Hotel.ID
		if err := tx.Create(&onboarding.PrimaryAdmin).Error; err != nil {
			return err
		}
		onboarding.PrimaryAdmin.Hotel = onboarding.Hotel
		services := append(models.CoreServices(onboarding.Hotel.ID), models.RevenueServices(onboarding.Hotel.ID)...)
		if err := tx.Create(&services).Error; err != nil {
			return err
		}
		facilities := models.DefaultFacilities(onboarding.Hotel.ID)
		if err := tx.Create(&facilities).Error; err != nil {
			return err
		}
		promotion := models.DefaultPromotion(onboarding.Hotel.ID, time.Now().UTC())
		if err := tx.Create(&promotion).Error; err != nil {
			return err
		}
		restaurant, items := models.DefaultRestaurant(onboarding.Hotel.ID)
		if err := tx.Create(&restaurant).Error; err != nil {
			return err
		}
		for index := range items {
			items[index].RestaurantID = restaurant.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		knowledge := models.DefaultKnowledge(onboarding.Hotel.ID, time.Now().UTC())
		return tx.Create(&knowledge).Error
	})
}

func (s *GORMStore) FindHotelBySlug(ctx context.Context, slug string) (models.Hotel, error) {
	var hotel models.Hotel
	err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&hotel).Error
	return hotel, err
}

func (s *GORMStore) FindStaffForLogin(ctx context.Context, hotelSlug, email string) (models.StaffUser, error) {
	var staff models.StaffUser
	err := s.db.WithContext(ctx).
		Model(&models.StaffUser{}).
		Joins("JOIN hotels ON hotels.id = staff_users.hotel_id AND hotels.deleted_at IS NULL").
		Where("hotels.slug = ? AND staff_users.email = ?", hotelSlug, email).
		Preload("Hotel").
		First(&staff).Error
	return staff, err
}

func (s *GORMStore) FindStaffByID(ctx context.Context, hotelID, staffID uuid.UUID) (models.StaffUser, error) {
	var staff models.StaffUser
	err := s.db.WithContext(ctx).
		Where("hotel_id = ? AND id = ? AND is_active = ?", hotelID, staffID, true).
		Preload("Hotel").
		First(&staff).Error
	return staff, err
}

func (s *GORMStore) MarkStaffLogin(ctx context.Context, hotelID, staffID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&models.StaffUser{}).
		Where("hotel_id = ? AND id = ?", hotelID, staffID).
		Update("last_login_at", at)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *GORMStore) CreateStaff(ctx context.Context, staff *models.StaffUser) error {
	return s.db.WithContext(ctx).Create(staff).Error
}

func (s *GORMStore) ListStaff(ctx context.Context, hotelID uuid.UUID) ([]models.StaffUser, error) {
	var staff []models.StaffUser
	err := s.db.WithContext(ctx).
		Where("hotel_id = ?", hotelID).
		Order("created_at ASC").
		Find(&staff).Error
	return staff, err
}

func (s *GORMStore) UpdateHotelBranding(ctx context.Context, hotelID uuid.UUID, name, logoURL, primaryColor, timezone string) (models.Hotel, error) {
	updates := map[string]any{"name": name, "logo_url": logoURL, "primary_color": primaryColor, "timezone": timezone}
	result := s.db.WithContext(ctx).Model(&models.Hotel{}).Where("id = ?", hotelID).Updates(updates)
	if result.Error != nil {
		return models.Hotel{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.Hotel{}, gorm.ErrRecordNotFound
	}
	var hotel models.Hotel
	err := s.db.WithContext(ctx).Where("id = ?", hotelID).First(&hotel).Error
	return hotel, err
}

func (s *GORMStore) FindActiveStayForGuestLogin(ctx context.Context, hotelSlug, roomNumber string) (models.Stay, error) {
	var stay models.Stay
	err := s.db.WithContext(ctx).
		Model(&models.Stay{}).
		Joins("JOIN hotels ON hotels.id = stays.hotel_id AND hotels.deleted_at IS NULL").
		Joins("JOIN rooms ON rooms.id = stays.room_id AND rooms.hotel_id = stays.hotel_id AND rooms.deleted_at IS NULL").
		Where("hotels.slug = ? AND rooms.number = ? AND stays.status = ?", hotelSlug, roomNumber, models.StayActive).
		Preload("Guest").
		Preload("Hotel").
		Preload("Room").
		Order("stays.created_at DESC").
		First(&stay).Error
	return stay, err
}

func (s *GORMStore) FindGuestSession(ctx context.Context, hotelID, guestID, stayID uuid.UUID) (models.Stay, error) {
	var stay models.Stay
	err := s.db.WithContext(ctx).
		Where("hotel_id = ? AND guest_id = ? AND id = ? AND status IN ?", hotelID, guestID, stayID, []models.StayStatus{models.StayPreArrival, models.StayActive}).
		Preload("Guest").
		Preload("Hotel").
		Preload("Room").
		Preload("Reservation.Guest").
		Preload("Reservation.Room").
		First(&stay).Error
	return stay, err
}

func (s *GORMStore) WriteAudit(ctx context.Context, audit *models.AuditLog) error {
	return s.db.WithContext(ctx).Create(audit).Error
}
