package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/TablazOrg/HotelMate/backend/internal/operations"
)

// This compatibility entrypoint intentionally delegates to the unified
// operations CLI so older automation cannot bypass its safety contract.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(operations.NewApp(os.Stdout, os.Stderr).Run(ctx, []string{"migrate", "up", "--yes"}))
}
