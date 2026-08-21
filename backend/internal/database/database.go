package database

import (
	"context"
	"fmt"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
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
