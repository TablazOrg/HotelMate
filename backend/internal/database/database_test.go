package database

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMigrationVersionsFitLedger(t *testing.T) {
	field, ok := reflect.TypeOf(schemaMigration{}).FieldByName("Version")
	if !ok {
		t.Fatal("schema migration version field is missing")
	}
	wantSizeTag := fmt.Sprintf("size:%d", schemaMigrationVersionSize)
	if !strings.Contains(field.Tag.Get("gorm"), wantSizeTag) {
		t.Fatalf("version field must declare %q in its GORM tag", wantSizeTag)
	}

	for _, migration := range migrationSteps {
		if len(migration.version) > schemaMigrationVersionSize {
			t.Errorf("migration version %q has length %d; ledger capacity is %d", migration.version, len(migration.version), schemaMigrationVersionSize)
		}
	}
}

func TestMigrateWidensLegacyLedgerAndIsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer Close(db)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	schemaName := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := tx.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schemaName)).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	if err := tx.Exec(fmt.Sprintf(`SET LOCAL search_path TO %q`, schemaName)).Error; err != nil {
		t.Fatalf("select isolated schema: %v", err)
	}
	if err := tx.Exec(`
		CREATE TABLE hotelmate_schema_migrations (
			version varchar(32) PRIMARY KEY,
			applied_at timestamptz NOT NULL
		)
	`).Error; err != nil {
		t.Fatalf("create legacy migration ledger: %v", err)
	}

	if err := Migrate(tx); err != nil {
		t.Fatalf("migrate legacy ledger: %v", err)
	}
	if err := Migrate(tx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	var maximumLength int64
	if err := tx.Raw(`
		SELECT character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = ?
		  AND table_name = 'hotelmate_schema_migrations'
		  AND column_name = 'version'
	`, schemaName).Scan(&maximumLength).Error; err != nil {
		t.Fatalf("read migration ledger width: %v", err)
	}
	if maximumLength != schemaMigrationVersionSize {
		t.Fatalf("migration ledger width = %d; want %d", maximumLength, schemaMigrationVersionSize)
	}

	var applied int64
	if err := tx.Model(&schemaMigration{}).Count(&applied).Error; err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != int64(len(migrationSteps)) {
		t.Fatalf("applied migrations = %d; want %d", applied, len(migrationSteps))
	}
}
