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

func TestCheckInInvitationIsSignedPurposeBoundAndExpiring(t *testing.T) {
	manager, _ := NewTokenManager("test-secret-that-is-at-least-thirty-two-characters", "hotelmate-test", time.Hour, time.Hour)
	clock := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return clock }
	reservationID, hotelID := uuid.New(), uuid.New()
	token, expiresAt, err := manager.IssueCheckInInvitation(reservationID, hotelID, 2*time.Hour)
	if err != nil || !expiresAt.Equal(clock.Add(2*time.Hour)) {
		t.Fatalf("issue invitation: expires=%s err=%v", expiresAt, err)
	}
	claims, err := manager.ParseCheckInInvitation(token)
	if err != nil || claims.Subject != reservationID.String() || claims.HotelID != hotelID.String() {
		t.Fatalf("parse invitation: claims=%+v err=%v", claims, err)
	}
	if _, err := manager.Parse(token, ActorGuest); err == nil {
		t.Fatal("invitation token must not authenticate as a guest session")
	}
	manager.now = func() time.Time { return clock.Add(3 * time.Hour) }
	if _, err := manager.ParseCheckInInvitation(token); err == nil {
		t.Fatal("expired invitation must be rejected")
	}
}
