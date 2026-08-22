package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/config"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	if cfg.Environment == "production" {
		logger.Error("demo seed is disabled in production")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer database.Close(db)
	if err := database.Migrate(db); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	settings := demoSettings{
		hotelSlug: env("DEMO_HOTEL_SLUG", "demo-hotel"), hotelName: env("DEMO_HOTEL_NAME", "HotelMate Demo"),
		adminEmail: env("DEMO_ADMIN_EMAIL", "admin@hotelmate.local"), adminPassword: env("DEMO_ADMIN_PASSWORD", "hotelmate-demo-admin"),
		roomNumber: env("DEMO_ROOM_NUMBER", "101"), guestIdentity: env("DEMO_GUEST_IDENTITY", "A1234567"),
	}
	if err := seed(db, settings); err != nil {
		logger.Error("seed demo", "error", err)
		os.Exit(1)
	}
	fmt.Printf("Demo ready\nHotel slug: %s\nStaff: %s / %s\nGuest: room %s / %s\n", settings.hotelSlug, settings.adminEmail, settings.adminPassword, settings.roomNumber, settings.guestIdentity)
}

type demoSettings struct {
	hotelSlug, hotelName, adminEmail, adminPassword, roomNumber, guestIdentity string
}

func seed(db *gorm.DB, settings demoSettings) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var hotel models.Hotel
		result := tx.Where("slug = ?", settings.hotelSlug).First(&hotel)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			return result.Error
		}
		if result.Error == gorm.ErrRecordNotFound {
			hotel = models.Hotel{Name: settings.hotelName, Slug: settings.hotelSlug, PrimaryColor: "#0f766e", Timezone: "Asia/Tehran"}
			if err := tx.Create(&hotel).Error; err != nil {
				return err
			}
		}
		services := append(models.CoreServices(hotel.ID), models.RevenueServices(hotel.ID)...)
		for _, service := range services {
			var count int64
			if err := tx.Model(&models.Service{}).Where("hotel_id = ? AND code = ?", hotel.ID, service.Code).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Create(&service).Error; err != nil {
					return err
				}
			}
		}

		var staff models.StaffUser
		err := tx.Where("hotel_id = ? AND email = ?", hotel.ID, strings.ToLower(settings.adminEmail)).First(&staff).Error
		if err == gorm.ErrRecordNotFound {
			hash, hashErr := auth.HashPassword(settings.adminPassword)
			if hashErr != nil {
				return hashErr
			}
			staff = models.StaffUser{HotelID: hotel.ID, FirstName: "Demo", LastName: "Admin", Email: strings.ToLower(settings.adminEmail), PasswordHash: hash, Role: models.StaffRolePrimaryAdmin, IsActive: true}
			if err := tx.Create(&staff).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		var room models.Room
		err = tx.Where("hotel_id = ? AND number = ?", hotel.ID, settings.roomNumber).First(&room).Error
		if err == gorm.ErrRecordNotFound {
			room = models.Room{HotelID: hotel.ID, Number: settings.roomNumber, Floor: 1, Type: "Double", Status: models.RoomStatusOccupied}
			if err := tx.Create(&room).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		var guest models.Guest
		err = tx.Where("hotel_id = ? AND first_name = ? AND last_name = ?", hotel.ID, "Demo", "Guest").First(&guest).Error
		if err == gorm.ErrRecordNotFound {
			hash, hashErr := auth.HashIdentity(settings.guestIdentity)
			if hashErr != nil {
				return hashErr
			}
			guest = models.Guest{HotelID: hotel.ID, FirstName: "Demo", LastName: "Guest", IdentityType: "passport", IdentityNumberHash: hash}
			if err := tx.Create(&guest).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		var activeStay models.Stay
		err = tx.Where("hotel_id = ? AND room_id = ? AND status = ?", hotel.ID, room.ID, models.StayActive).First(&activeStay).Error
		if err == gorm.ErrRecordNotFound {
			now := time.Now().UTC()
			activeStay = models.Stay{HotelID: hotel.ID, GuestID: guest.ID, RoomID: room.ID, Status: models.StayActive, CheckInAt: &now}
			if err := tx.Create(&activeStay).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if err := seedHotelContent(tx, hotel.ID); err != nil {
			return err
		}
		return nil
	})
}

func seedHotelContent(tx *gorm.DB, hotelID uuid.UUID) error {
	var facilityCount int64
	if err := tx.Model(&models.Facility{}).Where("hotel_id = ?", hotelID).Count(&facilityCount).Error; err != nil {
		return err
	}
	if facilityCount == 0 {
		facilities := models.DefaultFacilities(hotelID)
		if err := tx.Create(&facilities).Error; err != nil {
			return err
		}
	}

	var promotionCount int64
	if err := tx.Model(&models.Promotion{}).Where("hotel_id = ?", hotelID).Count(&promotionCount).Error; err != nil {
		return err
	}
	if promotionCount == 0 {
		promotion := models.DefaultPromotion(hotelID, time.Now().UTC())
		if err := tx.Create(&promotion).Error; err != nil {
			return err
		}
	}

	var restaurantCount int64
	if err := tx.Model(&models.Restaurant{}).Where("hotel_id = ?", hotelID).Count(&restaurantCount).Error; err != nil {
		return err
	}
	if restaurantCount == 0 {
		restaurant, items := models.DefaultRestaurant(hotelID)
		if err := tx.Create(&restaurant).Error; err != nil {
			return err
		}
		for index := range items {
			items[index].RestaurantID = restaurant.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
	}

	var knowledgeCount int64
	if err := tx.Model(&models.KnowledgeItem{}).Where("hotel_id = ?", hotelID).Count(&knowledgeCount).Error; err != nil {
		return err
	}
	if knowledgeCount == 0 {
		knowledge := models.DefaultKnowledge(hotelID, time.Now().UTC())
		if err := tx.Create(&knowledge).Error; err != nil {
			return err
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
