package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
)

func TestOperationalReportingAndAuditIsolationPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close(db)
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if !db.Migrator().HasColumn(&models.AuditLog{}, "RequestID") {
		t.Fatal("M6 request ID migration was not applied")
	}

	repository := store.New(db)
	suffix := uuid.NewString()[:8]
	onboarding := store.HotelOnboarding{
		Hotel:        models.Hotel{Name: "Report " + suffix, Slug: "report-" + suffix, PrimaryColor: "#f53d46", Timezone: "UTC"},
		PrimaryAdmin: models.StaffUser{FirstName: "Admin", LastName: suffix, Email: "report-" + suffix + "@example.com", PasswordHash: "unused-test-hash", Role: models.StaffRolePrimaryAdmin, IsActive: true},
	}
	if err := repository.CreateHotelWithPrimaryAdmin(ctx, &onboarding); err != nil {
		t.Fatalf("onboard reporting hotel: %v", err)
	}
	hotel := onboarding.Hotel
	services, err := repository.ListStaffServices(ctx, hotel.ID)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	var paid models.Service
	for _, service := range services {
		if service.IsPaid {
			paid = service
			break
		}
	}
	guest := models.Guest{HotelID: hotel.ID, FirstName: "Guest", LastName: suffix, IdentityType: "passport", IdentityNumberHash: "unused-test-hash"}
	room := models.Room{HotelID: hotel.ID, Number: "R-" + suffix, Status: models.RoomStatusOccupied}
	if err := db.Create(&guest).Error; err != nil {
		t.Fatalf("create guest: %v", err)
	}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	stay := models.Stay{HotelID: hotel.ID, GuestID: guest.ID, RoomID: room.ID, Status: models.StayActive}
	if err := db.Create(&stay).Error; err != nil {
		t.Fatalf("create stay: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	started, completed := now.Add(-30*time.Minute), now.Add(-10*time.Minute)
	request := models.ServiceRequest{
		BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Hour), UpdatedAt: completed}, HotelID: hotel.ID, StayID: stay.ID,
		ServiceID: paid.ID, Status: models.RequestCompleted, Quantity: 1, TotalPriceCents: paid.PriceCents, StartedAt: &started, CompletedAt: &completed,
	}
	if err := db.Create(&request).Error; err != nil {
		t.Fatalf("create completed paid request: %v", err)
	}
	audit := models.AuditLog{HotelID: &hotel.ID, ActorType: "staff", Action: "staff.login", Outcome: models.AuditOutcomeFailure, RequestID: "report-test-request", Metadata: json.RawMessage(`{"reason":"test"}`), CreatedAt: now}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatalf("create audit: %v", err)
	}

	from, to := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	report, err := repository.BuildOperationalReport(ctx, hotel.ID, from, to, time.UTC)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.Summary.RequestsCreated != 1 || report.Summary.CompletedRequests != 1 || report.Summary.RecognizedRevenueCents != paid.PriceCents || report.Summary.ActiveRooms != 1 || report.Summary.TotalRooms != 1 || report.Summary.FailedSecurityEvents != 1 {
		t.Fatalf("unexpected report summary: %+v", report.Summary)
	}
	page, err := repository.ListAuditLogs(ctx, hotel.ID, store.AuditFilter{From: from, To: to, Outcome: models.AuditOutcomeFailure, Limit: 10})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].RequestID != "report-test-request" {
		t.Fatalf("unexpected audit page: %+v err=%v", page, err)
	}
	otherPage, err := repository.ListAuditLogs(ctx, uuid.New(), store.AuditFilter{From: from, To: to, Limit: 10})
	if err != nil || otherPage.Total != 0 || len(otherPage.Items) != 0 {
		t.Fatalf("audit leaked across tenants: %+v err=%v", otherPage, err)
	}
}
