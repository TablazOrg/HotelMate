package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/config"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer database.Close(db)
	purged, err := store.New(db).PurgeExpiredMessages(ctx, time.Now().UTC())
	if err != nil {
		logger.Error("purge expired chat messages", "error", err)
		os.Exit(1)
	}
	logger.Info("chat message purge complete", "purged", purged)
}
