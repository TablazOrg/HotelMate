package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/config"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
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
	storage, err := documents.NewLocalStorage(cfg.UploadsDir, cfg.DocumentMaxBytes)
	if err != nil {
		logger.Error("initialize document storage", "error", err)
		os.Exit(1)
	}
	repository := store.New(db)
	purged := 0
	failed := 0
	for {
		items, err := repository.ListExpiredDocuments(ctx, time.Now().UTC(), 100)
		if err != nil {
			logger.Error("list expired documents", "error", err)
			os.Exit(1)
		}
		if len(items) == 0 {
			break
		}
		batchPurged := 0
		for _, item := range items {
			if err := storage.Delete(ctx, item.DocumentStorageKey); err != nil {
				logger.Error("delete expired document", "checkInId", item.ID, "error", err)
				failed++
				continue
			}
			if err := repository.MarkDocumentDeleted(ctx, item.ID, time.Now().UTC()); err != nil {
				logger.Error("mark expired document", "checkInId", item.ID, "error", err)
				failed++
				continue
			}
			purged++
			batchPurged++
		}
		if len(items) < 100 || batchPurged == 0 {
			break
		}
	}
	logger.Info("document purge complete", "purged", purged, "failed", failed)
	if failed > 0 {
		os.Exit(1)
	}
}
