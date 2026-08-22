package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestConversationAndKnowledgeWorkflowPostgres(t *testing.T) {
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

	repository := store.New(db)
	suffix := uuid.NewString()[:8]
	onboarding := store.HotelOnboarding{
		Hotel:        models.Hotel{Name: "Chat " + suffix, Slug: "chat-" + suffix, PrimaryColor: "#f53d46", Timezone: "Asia/Tehran"},
		PrimaryAdmin: models.StaffUser{FirstName: "Admin", LastName: suffix, Email: "chat-" + suffix + "@example.com", PasswordHash: "unused-test-hash", Role: models.StaffRolePrimaryAdmin, IsActive: true},
	}
	if err := repository.CreateHotelWithPrimaryAdmin(ctx, &onboarding); err != nil {
		t.Fatalf("onboard hotel: %v", err)
	}
	hotel, admin := onboarding.Hotel, onboarding.PrimaryAdmin
	guest := models.Guest{HotelID: hotel.ID, FirstName: "Guest", LastName: suffix, IdentityType: "passport", IdentityNumberHash: "unused-test-hash"}
	room := models.Room{HotelID: hotel.ID, Number: "C-" + suffix, Status: models.RoomStatusOccupied}
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
	stay.Guest, stay.Room, stay.Hotel = guest, room, hotel
	now := time.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(90 * 24 * time.Hour)
	conversation, err := repository.GetOrCreateGuestConversation(ctx, stay, now, expires)
	if err != nil || conversation.Status != models.ConversationAI || len(conversation.Messages) != 1 {
		t.Fatalf("create conversation: %+v err=%v", conversation, err)
	}
	conversation, err = repository.AddGuestMessage(ctx, stay, conversation.ID, "استخر چه ساعتی باز است؟", false, now.Add(time.Second), expires)
	if err != nil || len(conversation.Messages) != 2 {
		t.Fatalf("add guest message: %+v err=%v", conversation, err)
	}
	approved, err := repository.ListApprovedKnowledge(ctx, hotel.ID)
	if err != nil || len(approved) != 6 {
		t.Fatalf("seeded approved knowledge: count=%d err=%v", len(approved), err)
	}
	confidence := 0.9
	conversation, err = repository.AddAssistantReply(ctx, hotel.ID, conversation.ID, approved[0].Content, &confidence, &approved[0].ID, true, now.Add(2*time.Second), expires)
	if err != nil || conversation.Status != models.ConversationHandedOff || len(conversation.Messages) != 3 {
		t.Fatalf("assistant handoff: %+v err=%v", conversation, err)
	}
	conversation, err = repository.AddStaffMessage(ctx, hotel.ID, conversation.ID, admin.ID, "پذیرش پاسخ می‌دهد.", false, now.Add(3*time.Second), expires)
	if err != nil || conversation.AssignedToID == nil || *conversation.AssignedToID != admin.ID || len(conversation.Messages) != 4 {
		t.Fatalf("staff reply: %+v err=%v", conversation, err)
	}

	newVersion, err := repository.CreateKnowledgeVersion(ctx, hotel.ID, admin.ID, approved[0].Title, "پاسخ به‌روزشده", "پذیرش", &approved[0].ID)
	if err != nil || newVersion.Version != approved[0].Version+1 || newVersion.Status != models.KnowledgePending {
		t.Fatalf("create knowledge version: %+v err=%v", newVersion, err)
	}
	newVersion, err = repository.ReviewKnowledge(ctx, hotel.ID, newVersion.ID, admin.ID, models.KnowledgeApproved, "", now)
	if err != nil || newVersion.Status != models.KnowledgeApproved {
		t.Fatalf("approve knowledge version: %+v err=%v", newVersion, err)
	}
	approved, err = repository.ListApprovedKnowledge(ctx, hotel.ID)
	if err != nil || len(approved) != 6 {
		t.Fatalf("only latest approved versions should publish: count=%d err=%v", len(approved), err)
	}
	foundNew := false
	for _, item := range approved {
		if item.ID == newVersion.ID {
			foundNew = true
		}
		if item.ID == *newVersion.SupersedesID {
			t.Fatal("superseded approved version remained published")
		}
	}
	if !foundNew {
		t.Fatal("new approved version was not published")
	}
	if _, err := repository.GetStaffConversation(ctx, uuid.New(), conversation.ID, now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("conversation leaked across tenant: %v", err)
	}
}
