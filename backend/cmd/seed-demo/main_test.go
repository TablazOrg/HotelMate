package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSeedProvidesCompleteDemoAndIsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close(db)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schemaName := "seed_" + suffix
	if err := tx.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schemaName)).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	if err := tx.Exec(fmt.Sprintf(`SET LOCAL search_path TO %q`, schemaName)).Error; err != nil {
		t.Fatalf("select isolated schema: %v", err)
	}
	if err := database.Migrate(tx); err != nil {
		t.Fatalf("migrate isolated schema: %v", err)
	}

	settings := demoSettings{
		hotelSlug: "demo-" + suffix[:8], hotelName: "Complete Demo",
		adminEmail: "admin-" + suffix[:8] + "@example.com", adminPassword: "test-demo-password",
		roomNumber: "T-" + suffix[:6], guestIdentity: "PASS-" + suffix[:8],
	}
	if err := seed(tx, settings); err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	if err := seed(tx, settings); err != nil {
		t.Fatalf("repeat demo seed: %v", err)
	}

	var hotel models.Hotel
	if err := tx.Where("slug = ?", settings.hotelSlug).First(&hotel).Error; err != nil {
		t.Fatalf("find seeded hotel: %v", err)
	}
	assertHotelCount(t, tx, &models.Service{}, hotel.ID, 12)
	assertHotelCount(t, tx, &models.Facility{}, hotel.ID, int64(len(models.DefaultFacilities(hotel.ID))))
	assertHotelCount(t, tx, &models.Promotion{}, hotel.ID, 1)
	assertHotelCount(t, tx, &models.Restaurant{}, hotel.ID, 1)
	assertHotelCount(t, tx, &models.KnowledgeItem{}, hotel.ID, int64(len(models.DefaultKnowledge(hotel.ID, time.Now()))))

	var paidServices int64
	if err := tx.Model(&models.Service{}).Where("hotel_id = ? AND is_paid = ?", hotel.ID, true).Count(&paidServices).Error; err != nil {
		t.Fatalf("count paid services: %v", err)
	}
	if paidServices != int64(len(models.RevenueServices(hotel.ID))) {
		t.Fatalf("paid services = %d; want %d", paidServices, len(models.RevenueServices(hotel.ID)))
	}

	var menuItems int64
	if err := tx.Model(&models.MenuItem{}).
		Joins("JOIN restaurants ON restaurants.id = menu_items.restaurant_id").
		Where("restaurants.hotel_id = ?", hotel.ID).
		Count(&menuItems).Error; err != nil {
		t.Fatalf("count menu items: %v", err)
	}
	_, defaultMenu := models.DefaultRestaurant(hotel.ID)
	if menuItems != int64(len(defaultMenu)) {
		t.Fatalf("menu items = %d; want %d", menuItems, len(defaultMenu))
	}
}

func assertHotelCount(t *testing.T, db *gorm.DB, model any, hotelID uuid.UUID, want int64) {
	t.Helper()
	var got int64
	if err := db.Model(model).Where("hotel_id = ?", hotelID).Count(&got).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	if got != want {
		t.Fatalf("%T count = %d; want %d", model, got, want)
	}
}
