package realtime

import (
	"sync"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

type Event struct {
	Type            string                 `json:"type"`
	Payload         any                    `json:"payload"`
	EmittedAt       time.Time              `json:"emittedAt"`
	HotelID         uuid.UUID              `json:"-"`
	StayID          uuid.UUID              `json:"-"`
	Category        models.ServiceCategory `json:"-"`
	FulfillmentRole models.StaffRole       `json:"-"`
}

type Subscriber struct {
	ActorType auth.ActorType
	HotelID   uuid.UUID
	StayID    uuid.UUID
	Role      models.StaffRole
}

type subscription struct {
	principal Subscriber
	events    chan Event
}

type Hub struct {
	mu            sync.RWMutex
	subscriptions map[uuid.UUID]subscription
}

func NewHub() *Hub {
	return &Hub{subscriptions: make(map[uuid.UUID]subscription)}
}

func (h *Hub) Subscribe(principal Subscriber) (<-chan Event, func()) {
	id := uuid.New()
	events := make(chan Event, 32)
	h.mu.Lock()
	h.subscriptions[id] = subscription{principal: principal, events: events}
	h.mu.Unlock()
	return events, func() {
		h.mu.Lock()
		if subscription, ok := h.subscriptions[id]; ok {
			delete(h.subscriptions, id)
			close(subscription.events)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(event Event) {
	if event.EmittedAt.IsZero() {
		event.EmittedAt = time.Now().UTC()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, subscription := range h.subscriptions {
		if !canReceive(subscription.principal, event) {
			continue
		}
		select {
		case subscription.events <- event:
		default:
			// A slow client can recover from the persisted request history after
			// reconnecting; it must not block operational updates for other users.
		}
	}
}

func canReceive(principal Subscriber, event Event) bool {
	if principal.HotelID != event.HotelID {
		return false
	}
	if principal.ActorType == auth.ActorGuest {
		return principal.StayID == event.StayID
	}
	switch principal.Role {
	case models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations, models.StaffRoleReception:
		return true
	case models.StaffRoleHousekeeping:
		return event.FulfillmentRole == models.StaffRoleHousekeeping
	case models.StaffRoleFB:
		return event.FulfillmentRole == models.StaffRoleFB
	default:
		return false
	}
}
