package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/config"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
	"github.com/TablazOrg/HotelMate/backend/internal/httpapi"
	"github.com/TablazOrg/HotelMate/backend/internal/observability"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			logger.Error("database close failed", "error", closeErr)
		}
	}()

	if cfg.AutoMigrate {
		if err := database.Migrate(db); err != nil {
			logger.Error("database migration failed", "error", err)
			os.Exit(1)
		}
		logger.Info("database migrations complete")
	}
	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.StaffTokenTTL, cfg.GuestTokenTTL)
	if err != nil {
		logger.Error("initialize token manager", "error", err)
		os.Exit(1)
	}
	repository := store.New(db)
	realtimeHub := realtime.NewHub()
	documentStorage, err := documents.NewLocalStorage(cfg.UploadsDir, cfg.DocumentMaxBytes)
	if err != nil {
		logger.Error("initialize document storage", "error", err)
		os.Exit(1)
	}
	metrics := observability.New(cfg.APIVersion, cfg.ReleaseCommit, cfg.ReleaseImage)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			DB: db, Store: repository, Lifecycle: repository, ServiceOperations: repository, Content: repository, Conversations: repository, Reporting: repository,
			Documents: documentStorage, Realtime: realtimeHub, Tokens: tokens, Version: cfg.APIVersion,
			AllowedOrigins: cfg.AllowedOrigins, OnboardingToken: cfg.OnboardingToken, Logger: logger,
			DocumentMaxBytes: cfg.DocumentMaxBytes, DocumentRetention: cfg.DocumentRetention,
			ChatRetention: cfg.ChatRetention, ChatConfidence: cfg.ChatConfidence,
			EnableHSTS: cfg.EnableHSTS,
			Metrics:    metrics,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr, "environment", cfg.Environment, "release", cfg.APIVersion, "commit", cfg.ReleaseCommit, "image", cfg.ReleaseImage)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("api graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("api stopped")
	}
}
