package database

import (
	"context"
	"fmt"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type schemaMigration struct {
	Version   string    `gorm:"primaryKey;size:32"`
	AppliedAt time.Time `gorm:"not null"`
}

func (schemaMigration) TableName() string { return "hotelmate_schema_migrations" }

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	migrations := []struct {
		version string
		apply   func(*gorm.DB) error
	}{
		{version: "2026082101_identity_tenancy", apply: migrateIdentityTenancy},
		{version: "2026082102_reservation_lifecycle", apply: migrateReservationLifecycle},
		{version: "2026082103_service_operations", apply: migrateServiceOperations},
	}

	for _, migration := range migrations {
		var count int64
		if err := db.Model(&schemaMigration{}).Where("version = ?", migration.version).Count(&count).Error; err != nil {
			return fmt.Errorf("read migration ledger for %s: %w", migration.version, err)
		}
		if count > 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.apply(tx); err != nil {
				return fmt.Errorf("apply migration %s: %w", migration.version, err)
			}
			return tx.Create(&schemaMigration{Version: migration.version, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrateServiceOperations(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.Service{}, "Code") {
		// M2 installations can already contain services, so the new code must be
		// added as nullable, backfilled, and only then constrained.
		if err := db.Exec("ALTER TABLE services ADD COLUMN code varchar(64)").Error; err != nil {
			return err
		}
	}
	if err := db.Exec("UPDATE services SET code = 'legacy-' || replace(id::text, '-', '') WHERE code IS NULL OR btrim(code) = ''").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE services ALTER COLUMN code SET NOT NULL").Error; err != nil {
		return err
	}

	if !db.Migrator().HasColumn(&models.ServiceRequest{}, "HotelID") {
		if err := db.Exec("ALTER TABLE service_requests ADD COLUMN hotel_id uuid").Error; err != nil {
			return err
		}
	}
	if err := db.Exec(`
		UPDATE service_requests AS request
		SET hotel_id = stay.hotel_id
		FROM stays AS stay
		WHERE request.stay_id = stay.id AND request.hotel_id IS NULL
	`).Error; err != nil {
		return err
	}
	var requestsWithoutHotel int64
	if err := db.Table("service_requests").Where("hotel_id IS NULL").Count(&requestsWithoutHotel).Error; err != nil {
		return err
	}
	if requestsWithoutHotel > 0 {
		return fmt.Errorf("cannot backfill hotel_id for %d service requests", requestsWithoutHotel)
	}
	if err := db.Exec("ALTER TABLE service_requests ALTER COLUMN hotel_id SET NOT NULL").Error; err != nil {
		return err
	}

	var assignmentDataType string
	if err := db.Raw(`
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'service_requests'
		  AND column_name = 'assigned_to_id'
	`).Scan(&assignmentDataType).Error; err != nil {
		return err
	}
	if assignmentDataType != "" && assignmentDataType != "uuid" {
		if err := db.Exec("ALTER TABLE service_requests ALTER COLUMN assigned_to_id TYPE uuid USING NULLIF(assigned_to_id, '')::uuid").Error; err != nil {
			return err
		}
	}

	if err := db.AutoMigrate(&models.Service{}, &models.ServiceRequest{}, &models.ServiceRequestEvent{}); err != nil {
		return err
	}
	if err := backfillLegacyRequestEvents(db); err != nil {
		return err
	}
	var hotels []models.Hotel
	if err := db.Find(&hotels).Error; err != nil {
		return err
	}
	for _, hotel := range hotels {
		for _, service := range models.CoreServices(hotel.ID) {
			var count int64
			if err := db.Model(&models.Service{}).Where("hotel_id = ? AND code = ?", hotel.ID, service.Code).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := db.Create(&service).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func backfillLegacyRequestEvents(db *gorm.DB) error {
	type legacyRequest struct {
		ID        uuid.UUID
		HotelID   uuid.UUID
		GuestID   uuid.UUID
		CreatedAt time.Time
	}

	var requests []legacyRequest
	if err := db.Table("service_requests AS request").
		Select("request.id, request.hotel_id, stay.guest_id, request.created_at").
		Joins("JOIN stays AS stay ON stay.id = request.stay_id").
		Joins("LEFT JOIN service_request_events AS event ON event.request_id = request.id AND event.event_type = ?", models.RequestEventCreated).
		Where("event.id IS NULL").
		Scan(&requests).Error; err != nil {
		return err
	}
	for _, request := range requests {
		event := models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: request.CreatedAt},
			HotelID:   request.HotelID, RequestID: request.ID, EventType: models.RequestEventCreated,
			ActorType: "guest", ActorID: request.GuestID,
		}
		if err := db.Create(&event).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateReservationLifecycle(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Reservation{},
		&models.Stay{},
		&models.OnlineCheckIn{},
	)
}

func migrateIdentityTenancy(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Hotel{},
		&models.Guest{},
		&models.StaffUser{},
		&models.Room{},
		&models.Reservation{},
		&models.Stay{},
		&models.Service{},
		&models.ServiceRequest{},
		&models.ServiceRequestEvent{},
		&models.Facility{},
		&models.Promotion{},
		&models.Restaurant{},
		&models.MenuItem{},
		&models.KnowledgeItem{},
		&models.Conversation{},
		&models.Message{},
		&models.AuditLog{},
	)
}
