package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *BaseModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type Hotel struct {
	BaseModel
	Name         string `gorm:"not null" json:"name"`
	Slug         string `gorm:"not null;uniqueIndex" json:"slug"`
	LogoURL      string `json:"logoUrl"`
	PrimaryColor string `gorm:"size:20;not null;default:#0f766e" json:"primaryColor"`
	Timezone     string `gorm:"size:64;not null;default:Asia/Tehran" json:"timezone"`
}

type Guest struct {
	BaseModel
	HotelID            uuid.UUID `gorm:"type:uuid;not null;index" json:"hotelId"`
	Hotel              Hotel     `json:"-"`
	FirstName          string    `gorm:"not null" json:"firstName"`
	LastName           string    `gorm:"not null" json:"lastName"`
	IdentityType       string    `gorm:"size:20;not null" json:"identityType"`
	IdentityNumberHash string    `gorm:"not null" json:"-"`
	Phone              string    `json:"phone"`
}

type StaffRole string

const (
	StaffRolePrimaryAdmin   StaffRole = "primary_admin"
	StaffRoleSecondaryAdmin StaffRole = "secondary_admin"
	StaffRoleOperations     StaffRole = "operations_manager"
	StaffRoleReception      StaffRole = "reception"
	StaffRoleHousekeeping   StaffRole = "housekeeping"
	StaffRoleFB             StaffRole = "food_beverage"
)

type StaffUser struct {
	BaseModel
	HotelID      uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:uq_staff_hotel_email,priority:1" json:"hotelId"`
	Hotel        Hotel      `json:"-"`
	FirstName    string     `gorm:"not null" json:"firstName"`
	LastName     string     `gorm:"not null" json:"lastName"`
	Email        string     `gorm:"not null;uniqueIndex:uq_staff_hotel_email,priority:2" json:"email"`
	PasswordHash string     `gorm:"not null" json:"-"`
	Role         StaffRole  `gorm:"size:32;not null" json:"role"`
	IsActive     bool       `gorm:"not null;default:true" json:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
}

type RoomStatus string

const (
	RoomStatusAvailable RoomStatus = "available"
	RoomStatusOccupied  RoomStatus = "occupied"
	RoomStatusCleaning  RoomStatus = "cleaning"
	RoomStatusOutOfSvc  RoomStatus = "out_of_service"
)

type Room struct {
	BaseModel
	HotelID uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:uq_rooms_hotel_number,priority:1" json:"hotelId"`
	Hotel   Hotel      `json:"-"`
	Number  string     `gorm:"not null;uniqueIndex:uq_rooms_hotel_number,priority:2" json:"number"`
	Floor   int        `json:"floor"`
	Type    string     `gorm:"size:64" json:"type"`
	Status  RoomStatus `gorm:"size:24;not null;default:available" json:"status"`
}

type ReservationStatus string

const (
	ReservationPending   ReservationStatus = "pending"
	ReservationConfirmed ReservationStatus = "confirmed"
	ReservationCancelled ReservationStatus = "cancelled"
	ReservationCompleted ReservationStatus = "completed"
)

type Reservation struct {
	BaseModel
	HotelID          uuid.UUID         `gorm:"not null;index" json:"hotelId"`
	Hotel            Hotel             `json:"-"`
	GuestID          uuid.UUID         `gorm:"not null;index" json:"guestId"`
	Guest            Guest             `json:"guest"`
	RoomID           *uuid.UUID        `gorm:"index" json:"roomId"`
	ConfirmationCode string            `gorm:"not null;uniqueIndex" json:"confirmationCode"`
	Status           ReservationStatus `gorm:"size:24;not null;default:pending" json:"status"`
	ArrivalDate      time.Time         `gorm:"not null" json:"arrivalDate"`
	DepartureDate    time.Time         `gorm:"not null" json:"departureDate"`
}

type StayStatus string

const (
	StayPreArrival StayStatus = "pre_arrival"
	StayActive     StayStatus = "active"
	StayCheckedOut StayStatus = "checked_out"
)

type Stay struct {
	BaseModel
	HotelID       uuid.UUID  `gorm:"not null;index" json:"hotelId"`
	Hotel         Hotel      `json:"-"`
	GuestID       uuid.UUID  `gorm:"not null;index" json:"guestId"`
	Guest         Guest      `json:"guest"`
	RoomID        uuid.UUID  `gorm:"not null;index" json:"roomId"`
	Room          Room       `json:"room"`
	ReservationID *uuid.UUID `gorm:"index" json:"reservationId"`
	Status        StayStatus `gorm:"size:24;not null;default:pre_arrival" json:"status"`
	CheckInAt     *time.Time `json:"checkInAt"`
	CheckOutAt    *time.Time `json:"checkOutAt"`
}

type ServiceCategory string

const (
	ServiceCategoryHousekeeping ServiceCategory = "housekeeping"
	ServiceCategoryFNB          ServiceCategory = "food_beverage"
	ServiceCategoryTransport    ServiceCategory = "transport"
	ServiceCategoryWellness     ServiceCategory = "wellness"
	ServiceCategoryOther        ServiceCategory = "other"
)

type Service struct {
	BaseModel
	HotelID     uuid.UUID       `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel           `json:"-"`
	Name        string          `gorm:"not null" json:"name"`
	Description string          `json:"description"`
	Category    ServiceCategory `gorm:"size:32;not null" json:"category"`
	PriceCents  int64           `gorm:"not null;default:0" json:"priceCents"`
	Currency    string          `gorm:"size:3;not null;default:IRR" json:"currency"`
	IsPaid      bool            `gorm:"not null;default:false" json:"isPaid"`
	IsActive    bool            `gorm:"not null;default:true" json:"isActive"`
}

type RequestStatus string

const (
	RequestNew        RequestStatus = "new"
	RequestInProgress RequestStatus = "in_progress"
	RequestCompleted  RequestStatus = "completed"
	RequestCancelled  RequestStatus = "cancelled"
)

type ServiceRequest struct {
	BaseModel
	StayID          uuid.UUID     `gorm:"not null;index" json:"stayId"`
	Stay            Stay          `json:"-"`
	ServiceID       uuid.UUID     `gorm:"not null;index" json:"serviceId"`
	Service         Service       `json:"service"`
	AssignedToID    *uuid.UUID    `gorm:"index" json:"assignedToId"`
	Status          RequestStatus `gorm:"size:24;not null;default:new;index" json:"status"`
	Priority        int           `gorm:"not null;default:0" json:"priority"`
	Quantity        int           `gorm:"not null;default:1" json:"quantity"`
	Notes           string        `json:"notes"`
	TotalPriceCents int64         `gorm:"not null;default:0" json:"totalPriceCents"`
	CompletedAt     *time.Time    `json:"completedAt"`
}

type Facility struct {
	BaseModel
	HotelID     uuid.UUID `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel     `json:"-"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `gorm:"not null;default:true" json:"isActive"`
}

type Promotion struct {
	BaseModel
	HotelID     uuid.UUID `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel     `json:"-"`
	Title       string    `gorm:"not null" json:"title"`
	Description string    `json:"description"`
	DiscountPct float64   `gorm:"not null;default:0" json:"discountPct"`
	StartsAt    time.Time `gorm:"not null" json:"startsAt"`
	EndsAt      time.Time `gorm:"not null" json:"endsAt"`
	IsActive    bool      `gorm:"not null;default:true" json:"isActive"`
}

type Restaurant struct {
	BaseModel
	HotelID     uuid.UUID `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel     `json:"-"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `gorm:"not null;default:true" json:"isActive"`
}

type MenuItem struct {
	BaseModel
	RestaurantID uuid.UUID  `gorm:"not null;index" json:"restaurantId"`
	Restaurant   Restaurant `json:"-"`
	Name         string     `gorm:"not null" json:"name"`
	Description  string     `json:"description"`
	PriceCents   int64      `gorm:"not null;default:0" json:"priceCents"`
	IsAvailable  bool       `gorm:"not null;default:true" json:"isAvailable"`
}

type KnowledgeStatus string

const (
	KnowledgeDraft    KnowledgeStatus = "draft"
	KnowledgePending  KnowledgeStatus = "pending_review"
	KnowledgeApproved KnowledgeStatus = "approved"
	KnowledgeRejected KnowledgeStatus = "rejected"
)

type KnowledgeItem struct {
	BaseModel
	HotelID      uuid.UUID       `gorm:"not null;index" json:"hotelId"`
	Hotel        Hotel           `json:"-"`
	Title        string          `gorm:"not null" json:"title"`
	Content      string          `gorm:"not null;type:text" json:"content"`
	Status       KnowledgeStatus `gorm:"size:24;not null;default:draft;index" json:"status"`
	ApprovedByID *uuid.UUID      `gorm:"index" json:"approvedById"`
	ApprovedAt   *time.Time      `json:"approvedAt"`
}

type ConversationStatus string

const (
	ConversationAI        ConversationStatus = "ai"
	ConversationHandedOff ConversationStatus = "handed_off"
	ConversationClosed    ConversationStatus = "closed"
)

type Conversation struct {
	BaseModel
	HotelID uuid.UUID          `gorm:"not null;index" json:"hotelId"`
	Hotel   Hotel              `json:"-"`
	GuestID uuid.UUID          `gorm:"not null;index" json:"guestId"`
	Guest   Guest              `json:"guest"`
	StayID  *uuid.UUID         `gorm:"index" json:"stayId"`
	Status  ConversationStatus `gorm:"size:24;not null;default:ai" json:"status"`
}

type MessageRole string

const (
	MessageGuest  MessageRole = "guest"
	MessageAI     MessageRole = "ai"
	MessageStaff  MessageRole = "staff"
	MessageSystem MessageRole = "system"
)

type Message struct {
	BaseModel
	ConversationID uuid.UUID    `gorm:"not null;index" json:"conversationId"`
	Conversation   Conversation `json:"-"`
	Role           MessageRole  `gorm:"size:16;not null" json:"role"`
	Body           string       `gorm:"not null;type:text" json:"body"`
	Confidence     *float64     `json:"confidence,omitempty"`
}

type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "success"
	AuditOutcomeFailure AuditOutcome = "failure"
)

// AuditLog records security-sensitive actions without storing credentials or
// identity numbers. Metadata must only contain privacy-safe operational data.
type AuditLog struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time       `gorm:"not null;index" json:"createdAt"`
	HotelID   *uuid.UUID      `gorm:"type:uuid;index" json:"hotelId,omitempty"`
	ActorID   *uuid.UUID      `gorm:"type:uuid;index" json:"actorId,omitempty"`
	ActorType string          `gorm:"size:20;not null;index" json:"actorType"`
	Action    string          `gorm:"size:80;not null;index" json:"action"`
	Outcome   AuditOutcome    `gorm:"size:16;not null;index" json:"outcome"`
	IPAddress string          `gorm:"size:64" json:"ipAddress"`
	Metadata  json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
}

func (m *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if len(m.Metadata) == 0 {
		m.Metadata = json.RawMessage(`{}`)
	}
	return nil
}
