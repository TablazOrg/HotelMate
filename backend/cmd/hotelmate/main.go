package main

import (
	"context"
	"os"

	"github.com/TablazOrg/HotelMate/backend/internal/operations"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	app := operations.NewApp(os.Stdout, os.Stderr)
	app.Version = version
	app.Commit = commit
	app.BuildDate = buildDate
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}
