package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/TablazOrg/HotelMate/backend/internal/operations"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app := operations.NewApp(os.Stdout, os.Stderr)
	app.Version = version
	app.Commit = commit
	app.BuildDate = buildDate
	os.Exit(app.Run(ctx, os.Args[1:]))
}
