package store

import (
	"context"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HotelContent struct {
	Hotel       models.Hotel
	Facilities  []models.Facility
	Promotions  []models.Promotion
	Restaurants []models.Restaurant
}

type ContentStore interface {
	GetPublicContent(context.Context, string, time.Time) (HotelContent, error)
	GetStaffContent(context.Context, uuid.UUID) (HotelContent, error)
	CreateFacility(context.Context, *models.Facility) error
	UpdateFacility(context.Context, uuid.UUID, uuid.UUID, models.Facility) (models.Facility, error)
	CreatePromotion(context.Context, *models.Promotion) error
	UpdatePromotion(context.Context, uuid.UUID, uuid.UUID, models.Promotion) (models.Promotion, error)
	CreateRestaurant(context.Context, *models.Restaurant) error
	UpdateRestaurant(context.Context, uuid.UUID, uuid.UUID, models.Restaurant) (models.Restaurant, error)
	CreateMenuItem(context.Context, uuid.UUID, uuid.UUID, *models.MenuItem) error
	UpdateMenuItem(context.Context, uuid.UUID, uuid.UUID, models.MenuItem) (models.MenuItem, error)
}

func (s *GORMStore) GetPublicContent(ctx context.Context, slug string, now time.Time) (HotelContent, error) {
	var content HotelContent
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&content.Hotel).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ? AND is_active = ?", content.Hotel.ID, true).
		Order("sort_order ASC, name ASC").Find(&content.Facilities).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ? AND is_active = ? AND starts_at <= ? AND ends_at >= ?", content.Hotel.ID, true, now, now).
		Order("starts_at DESC").Find(&content.Promotions).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ? AND is_active = ?", content.Hotel.ID, true).
		Preload("MenuItems", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_available = ?", true).Order("sort_order ASC, name ASC")
		}).
		Order("sort_order ASC, name ASC").Find(&content.Restaurants).Error; err != nil {
		return content, err
	}
	return content, nil
}

func (s *GORMStore) GetStaffContent(ctx context.Context, hotelID uuid.UUID) (HotelContent, error) {
	var content HotelContent
	if err := s.db.WithContext(ctx).Where("id = ?", hotelID).First(&content.Hotel).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).Order("sort_order ASC, name ASC").Find(&content.Facilities).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).Order("starts_at DESC").Find(&content.Promotions).Error; err != nil {
		return content, err
	}
	if err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).
		Preload("MenuItems", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, name ASC") }).
		Order("sort_order ASC, name ASC").Find(&content.Restaurants).Error; err != nil {
		return content, err
	}
	return content, nil
}

func (s *GORMStore) CreateFacility(ctx context.Context, facility *models.Facility) error {
	return s.db.WithContext(ctx).Create(facility).Error
}

func (s *GORMStore) UpdateFacility(ctx context.Context, hotelID, id uuid.UUID, input models.Facility) (models.Facility, error) {
	updates := map[string]any{"name": input.Name, "description": input.Description, "icon": input.Icon, "hours": input.Hours, "sort_order": input.SortOrder, "is_active": input.IsActive}
	result := s.db.WithContext(ctx).Model(&models.Facility{}).Where("hotel_id = ? AND id = ?", hotelID, id).Updates(updates)
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return models.Facility{}, result.Error
		}
		return models.Facility{}, gorm.ErrRecordNotFound
	}
	var facility models.Facility
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, id).First(&facility).Error
	return facility, err
}

func (s *GORMStore) CreatePromotion(ctx context.Context, promotion *models.Promotion) error {
	return s.db.WithContext(ctx).Create(promotion).Error
}

func (s *GORMStore) UpdatePromotion(ctx context.Context, hotelID, id uuid.UUID, input models.Promotion) (models.Promotion, error) {
	updates := map[string]any{"title": input.Title, "description": input.Description, "discount_pct": input.DiscountPct, "badge_text": input.BadgeText, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "is_active": input.IsActive}
	result := s.db.WithContext(ctx).Model(&models.Promotion{}).Where("hotel_id = ? AND id = ?", hotelID, id).Updates(updates)
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return models.Promotion{}, result.Error
		}
		return models.Promotion{}, gorm.ErrRecordNotFound
	}
	var promotion models.Promotion
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, id).First(&promotion).Error
	return promotion, err
}

func (s *GORMStore) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) error {
	return s.db.WithContext(ctx).Create(restaurant).Error
}

func (s *GORMStore) UpdateRestaurant(ctx context.Context, hotelID, id uuid.UUID, input models.Restaurant) (models.Restaurant, error) {
	updates := map[string]any{"name": input.Name, "description": input.Description, "hours": input.Hours, "sort_order": input.SortOrder, "is_active": input.IsActive}
	result := s.db.WithContext(ctx).Model(&models.Restaurant{}).Where("hotel_id = ? AND id = ?", hotelID, id).Updates(updates)
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return models.Restaurant{}, result.Error
		}
		return models.Restaurant{}, gorm.ErrRecordNotFound
	}
	var restaurant models.Restaurant
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, id).Preload("MenuItems").First(&restaurant).Error
	return restaurant, err
}

func (s *GORMStore) CreateMenuItem(ctx context.Context, hotelID, restaurantID uuid.UUID, item *models.MenuItem) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var restaurant models.Restaurant
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("hotel_id = ? AND id = ?", hotelID, restaurantID).First(&restaurant).Error; err != nil {
			return err
		}
		item.RestaurantID = restaurant.ID
		return tx.Create(item).Error
	})
}

func (s *GORMStore) UpdateMenuItem(ctx context.Context, hotelID, id uuid.UUID, input models.MenuItem) (models.MenuItem, error) {
	var item models.MenuItem
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN restaurants ON restaurants.id = menu_items.restaurant_id AND restaurants.hotel_id = ?", hotelID).
			Where("menu_items.id = ?", id).First(&item).Error; err != nil {
			return err
		}
		return tx.Model(&item).Updates(map[string]any{"name": input.Name, "description": input.Description, "price_cents": input.PriceCents, "currency": input.Currency, "sort_order": input.SortOrder, "is_available": input.IsAvailable}).Error
	})
	if err != nil {
		return models.MenuItem{}, err
	}
	err = s.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	return item, err
}

var _ ContentStore = (*GORMStore)(nil)
