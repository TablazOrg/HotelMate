package auth

import (
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

func TestActorAudiencesAreSeparated(t *testing.T) {
	manager, err := NewTokenManager("test-secret-that-is-at-least-thirty-two-characters", "hotelmate-test", time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: uuid.New(), Role: models.StaffRoleReception}
	token, _, err := manager.IssueStaff(staff)
	if err != nil {
		t.Fatalf("issue staff token: %v", err)
	}
	claims, err := manager.Parse(token, ActorStaff)
	if err != nil {
		t.Fatalf("parse staff token: %v", err)
	}
	if claims.HotelID != staff.HotelID.String() || claims.Role != string(staff.Role) {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if _, err := manager.Parse(token, ActorGuest); err == nil {
		t.Fatal("staff token must not authenticate as guest")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	manager, _ := NewTokenManager("test-secret-that-is-at-least-thirty-two-characters", "hotelmate-test", time.Minute, time.Minute)
	clock := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return clock }
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: uuid.New(), Role: models.StaffRoleReception}
	token, _, _ := manager.IssueStaff(staff)
	manager.now = func() time.Time { return clock.Add(2 * time.Minute) }
	if _, err := manager.Parse(token, ActorStaff); err == nil {
		t.Fatal("expired token must be rejected")
	}
}
