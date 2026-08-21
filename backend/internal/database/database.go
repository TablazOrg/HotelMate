package database

import (
	"context"
	"fmt"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
	)
}
