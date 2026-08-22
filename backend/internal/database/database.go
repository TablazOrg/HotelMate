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

const schemaMigrationVersionSize = 128

type schemaMigration struct {
	Version   string    `gorm:"primaryKey;size:128"`
	AppliedAt time.Time `gorm:"not null"`
}

func (schemaMigration) TableName() string { return "hotelmate_schema_migrations" }

type migrationStep struct {
	version string
	apply   func(*gorm.DB) error
}

// MigrationStatus is the stable, secret-free representation consumed by the
// operations CLI and release evidence.
type MigrationStatus struct {
	Version   string     `json:"version"`
	Applied   bool       `json:"applied"`
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
}

var migrationSteps = []migrationStep{
	{version: "2026082101_identity_tenancy", apply: migrateIdentityTenancy},
	{version: "2026082102_reservation_lifecycle", apply: migrateReservationLifecycle},
	{version: "2026082103_service_operations", apply: migrateServiceOperations},
	{version: "2026082204_revenue_content", apply: migrateRevenueContent},
	{version: "2026082205_conversations_knowledge", apply: migrateConversationsKnowledge},
	{version: "2026082206_reporting_hardening", apply: migrateReportingHardening},
}

func MigrationVersions() []string {
	versions := make([]string, 0, len(migrationSteps))
	for _, step := range migrationSteps {
		versions = append(versions, step.version)
	}
	return versions
}

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
	for _, migration := range migrationSteps {
		if len(migration.version) > schemaMigrationVersionSize {
			return fmt.Errorf("migration version %q exceeds ledger capacity of %d characters", migration.version, schemaMigrationVersionSize)
		}
	}

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	if err := widenMigrationLedgerVersion(db); err != nil {
		return err
	}

	for _, migration := range migrationSteps {
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

// MigrationStatuses reports every version known to this binary. A database
// with no ledger is treated as a clean, fully pending target.
func MigrationStatuses(db *gorm.DB) ([]MigrationStatus, error) {
	statuses := make([]MigrationStatus, 0, len(migrationSteps))
	if !db.Migrator().HasTable(&schemaMigration{}) {
		for _, step := range migrationSteps {
			statuses = append(statuses, MigrationStatus{Version: step.version})
		}
		return statuses, nil
	}
	var applied []schemaMigration
	if err := db.Order("applied_at ASC").Find(&applied).Error; err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	byVersion := make(map[string]time.Time, len(applied))
	for _, item := range applied {
		byVersion[item.Version] = item.AppliedAt.UTC()
	}
	for _, step := range migrationSteps {
		status := MigrationStatus{Version: step.version}
		if appliedAt, ok := byVersion[step.version]; ok {
			status.Applied = true
			status.AppliedAt = &appliedAt
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func widenMigrationLedgerVersion(db *gorm.DB) error {
	var column struct {
		DataType      string `gorm:"column:data_type"`
		MaximumLength *int64 `gorm:"column:character_maximum_length"`
	}
	if err := db.Raw(`
		SELECT data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'hotelmate_schema_migrations'
		  AND column_name = 'version'
	`).Scan(&column).Error; err != nil {
		return fmt.Errorf("inspect migration ledger version column: %w", err)
	}
	if column.DataType == "" {
		return fmt.Errorf("inspect migration ledger version column: column not found")
	}
	if column.MaximumLength == nil || *column.MaximumLength >= schemaMigrationVersionSize {
		return nil
	}
	statement := fmt.Sprintf(
		"ALTER TABLE hotelmate_schema_migrations ALTER COLUMN version TYPE varchar(%d)",
		schemaMigrationVersionSize,
	)
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("widen migration ledger version column: %w", err)
	}
	return nil
}

func migrateReportingHardening(db *gorm.DB) error {
	return db.AutoMigrate(&models.AuditLog{})
}

func migrateConversationsKnowledge(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.KnowledgeItem{}, &models.Conversation{}, &models.Message{}); err != nil {
		return err
	}
	var hotels []models.Hotel
	if err := db.Find(&hotels).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, hotel := range hotels {
		var count int64
		if err := db.Model(&models.KnowledgeItem{}).Where("hotel_id = ?", hotel.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			items := models.DefaultKnowledge(hotel.ID, now)
			if err := db.Create(&items).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateRevenueContent(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.Service{}, &models.ServiceRequest{}, &models.Facility{}, &models.Promotion{}, &models.Restaurant{}, &models.MenuItem{}); err != nil {
		return err
	}
	var hotels []models.Hotel
	if err := db.Find(&hotels).Error; err != nil {
		return err
	}
	for _, hotel := range hotels {
		for _, service := range models.RevenueServices(hotel.ID) {
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
		var facilityCount int64
		if err := db.Model(&models.Facility{}).Where("hotel_id = ?", hotel.ID).Count(&facilityCount).Error; err != nil {
			return err
		}
		if facilityCount == 0 {
			facilities := models.DefaultFacilities(hotel.ID)
			if err := db.Create(&facilities).Error; err != nil {
				return err
			}
		}
		var promotionCount int64
		if err := db.Model(&models.Promotion{}).Where("hotel_id = ?", hotel.ID).Count(&promotionCount).Error; err != nil {
			return err
		}
		if promotionCount == 0 {
			promotion := models.DefaultPromotion(hotel.ID, time.Now().UTC())
			if err := db.Create(&promotion).Error; err != nil {
				return err
			}
		}
		var restaurantCount int64
		if err := db.Model(&models.Restaurant{}).Where("hotel_id = ?", hotel.ID).Count(&restaurantCount).Error; err != nil {
			return err
		}
		if restaurantCount == 0 {
			restaurant, items := models.DefaultRestaurant(hotel.ID)
			if err := db.Create(&restaurant).Error; err != nil {
				return err
			}
			for index := range items {
				items[index].RestaurantID = restaurant.ID
			}
			if err := db.Create(&items).Error; err != nil {
				return err
			}
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
