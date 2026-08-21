package realtime

import (
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

func TestHubScopesEventsByTenantStayAndRole(t *testing.T) {
	hub := NewHub()
	hotelID, otherHotelID, stayID := uuid.New(), uuid.New(), uuid.New()
	guestEvents, unsubscribeGuest := hub.Subscribe(Subscriber{ActorType: auth.ActorGuest, HotelID: hotelID, StayID: stayID})
	defer unsubscribeGuest()
	housekeepingEvents, unsubscribeHousekeeping := hub.Subscribe(Subscriber{ActorType: auth.ActorStaff, HotelID: hotelID, Role: models.StaffRoleHousekeeping})
	defer unsubscribeHousekeeping()
	otherTenantEvents, unsubscribeOther := hub.Subscribe(Subscriber{ActorType: auth.ActorStaff, HotelID: otherHotelID, Role: models.StaffRolePrimaryAdmin})
	defer unsubscribeOther()

	hub.Publish(Event{Type: "request.created", HotelID: hotelID, StayID: stayID, Category: models.ServiceCategoryOther, FulfillmentRole: models.StaffRoleHousekeeping})
	expectEvent(t, guestEvents)
	expectEvent(t, housekeepingEvents)
	expectNoEvent(t, otherTenantEvents)

	hub.Publish(Event{Type: "request.created", HotelID: hotelID, StayID: uuid.New(), Category: models.ServiceCategoryHousekeeping, FulfillmentRole: models.StaffRoleFB})
	expectNoEvent(t, guestEvents)
	expectNoEvent(t, housekeepingEvents)
}

func expectEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected realtime event")
	}
}

func expectNoEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected realtime event: %+v", event)
	case <-time.After(25 * time.Millisecond):
	}
}
