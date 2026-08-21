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
	HotelID          uuid.UUID         `gorm:"type:uuid;not null;index" json:"hotelId"`
	Hotel            Hotel             `json:"-"`
	GuestID          uuid.UUID         `gorm:"not null;index" json:"guestId"`
	Guest            Guest             `json:"guest"`
	RoomID           *uuid.UUID        `gorm:"index" json:"roomId"`
	Room             *Room             `json:"room,omitempty"`
	ConfirmationCode string            `gorm:"not null;uniqueIndex" json:"confirmationCode"`
	Status           ReservationStatus `gorm:"size:24;not null;default:pending" json:"status"`
	ArrivalDate      time.Time         `gorm:"not null" json:"arrivalDate"`
	DepartureDate    time.Time         `gorm:"not null" json:"departureDate"`
	ConfirmedAt      *time.Time        `json:"confirmedAt"`
	CancelledAt      *time.Time        `json:"cancelledAt"`
	Stay             *Stay             `gorm:"foreignKey:ReservationID" json:"stay,omitempty"`
}

type StayStatus string

const (
	StayPreArrival StayStatus = "pre_arrival"
	StayActive     StayStatus = "active"
	StayCheckedOut StayStatus = "checked_out"
)

type Stay struct {
	BaseModel
	HotelID       uuid.UUID    `gorm:"not null;index" json:"hotelId"`
	Hotel         Hotel        `json:"-"`
	GuestID       uuid.UUID    `gorm:"not null;index" json:"guestId"`
	Guest         Guest        `json:"guest"`
	RoomID        uuid.UUID    `gorm:"not null;index" json:"roomId"`
	Room          Room         `json:"room"`
	ReservationID *uuid.UUID   `gorm:"index;uniqueIndex" json:"reservationId"`
	Reservation   *Reservation `json:"reservation,omitempty"`
	Status        StayStatus   `gorm:"size:24;not null;default:pre_arrival" json:"status"`
	CheckInAt     *time.Time   `json:"checkInAt"`
	CheckOutAt    *time.Time   `json:"checkOutAt"`
}

type OnlineCheckInStatus string

const (
	OnlineCheckInSubmitted OnlineCheckInStatus = "submitted"
	OnlineCheckInApproved  OnlineCheckInStatus = "approved"
	OnlineCheckInRejected  OnlineCheckInStatus = "rejected"
)

// OnlineCheckIn stores only private document metadata. The document bytes live
// outside the web root and are available only through an authenticated staff
// endpoint until the retention deadline is reached.
type OnlineCheckIn struct {
	BaseModel
	HotelID            uuid.UUID           `gorm:"type:uuid;not null;index" json:"hotelId"`
	StayID             uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex" json:"stayId"`
	Stay               Stay                `json:"-"`
	Status             OnlineCheckInStatus `gorm:"size:24;not null;default:submitted;index" json:"status"`
	DocumentStorageKey string              `gorm:"not null;uniqueIndex" json:"-"`
	DocumentName       string              `gorm:"size:180;not null" json:"documentName"`
	DocumentMediaType  string              `gorm:"size:64;not null" json:"documentMediaType"`
	DocumentSize       int64               `gorm:"not null" json:"documentSize"`
	DocumentSHA256     string              `gorm:"size:64;not null" json:"-"`
	SubmittedAt        time.Time           `gorm:"not null" json:"submittedAt"`
	ReviewedAt         *time.Time          `json:"reviewedAt"`
	ReviewedByID       *uuid.UUID          `gorm:"type:uuid;index" json:"reviewedById"`
	ReviewNote         string              `gorm:"size:500" json:"reviewNote"`
	RetentionUntil     time.Time           `gorm:"not null;index" json:"retentionUntil"`
	DocumentDeletedAt  *time.Time          `gorm:"index" json:"documentDeletedAt"`
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
	HotelID          uuid.UUID       `gorm:"type:uuid;not null;index;uniqueIndex:uq_services_hotel_code,priority:1" json:"hotelId"`
	Hotel            Hotel           `json:"-"`
	Code             string          `gorm:"size:64;not null;uniqueIndex:uq_services_hotel_code,priority:2" json:"code"`
	Name             string          `gorm:"not null" json:"name"`
	Description      string          `json:"description"`
	Category         ServiceCategory `gorm:"size:32;not null" json:"category"`
	Icon             string          `gorm:"size:32;not null;default:concierge" json:"icon"`
	FulfillmentRole  StaffRole       `gorm:"size:32;not null;default:reception" json:"fulfillmentRole"`
	EstimatedMinutes int             `gorm:"not null;default:30" json:"estimatedMinutes"`
	PriceCents       int64           `gorm:"not null;default:0" json:"priceCents"`
	Currency         string          `gorm:"size:3;not null;default:IRR" json:"currency"`
	IsPaid           bool            `gorm:"not null;default:false" json:"isPaid"`
	IsQuickAction    bool            `gorm:"not null;default:false;index" json:"isQuickAction"`
	SortOrder        int             `gorm:"not null;default:0" json:"sortOrder"`
	IsActive         bool            `gorm:"not null;default:true" json:"isActive"`
}

func CoreServices(hotelID uuid.UUID) []Service {
	return []Service{
		{HotelID: hotelID, Code: "room-cleaning", Name: "نظافت اتاق", Description: "رسیدگی خانه‌داری و مرتب‌سازی اتاق", Category: ServiceCategoryHousekeeping, Icon: "cleaning", FulfillmentRole: StaffRoleHousekeeping, EstimatedMinutes: 30, Currency: "IRR", IsQuickAction: true, SortOrder: 10, IsActive: true},
		{HotelID: hotelID, Code: "mineral-water", Name: "آب معدنی", Description: "تحویل آب معدنی به اتاق", Category: ServiceCategoryHousekeeping, Icon: "water", FulfillmentRole: StaffRoleHousekeeping, EstimatedMinutes: 15, Currency: "IRR", IsQuickAction: true, SortOrder: 20, IsActive: true},
		{HotelID: hotelID, Code: "tea-coffee", Name: "چای و قهوه", Description: "سفارش نوشیدنی گرم برای اتاق", Category: ServiceCategoryFNB, Icon: "coffee", FulfillmentRole: StaffRoleFB, EstimatedMinutes: 20, Currency: "IRR", IsQuickAction: true, SortOrder: 30, IsActive: true},
		{HotelID: hotelID, Code: "amenities", Name: "لوازم بهداشتی", Description: "حوله و اقلام بهداشتی مصرفی", Category: ServiceCategoryHousekeeping, Icon: "amenities", FulfillmentRole: StaffRoleHousekeeping, EstimatedMinutes: 20, Currency: "IRR", IsQuickAction: true, SortOrder: 40, IsActive: true},
		{HotelID: hotelID, Code: "late-checkout", Name: "خروج دیرهنگام", Description: "بررسی امکان تمدید زمان خروج", Category: ServiceCategoryOther, Icon: "clock", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 15, Currency: "IRR", IsQuickAction: true, SortOrder: 50, IsActive: true},
		{HotelID: hotelID, Code: "transfer", Name: "ترانسفر", Description: "هماهنگی رفت‌وآمد فرودگاه یا ترمینال", Category: ServiceCategoryTransport, Icon: "car", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 30, Currency: "IRR", IsQuickAction: true, SortOrder: 60, IsActive: true},
	}
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
	HotelID         uuid.UUID             `gorm:"type:uuid;not null;index" json:"hotelId"`
	StayID          uuid.UUID             `gorm:"not null;index" json:"stayId"`
	Stay            Stay                  `json:"-"`
	ServiceID       uuid.UUID             `gorm:"not null;index" json:"serviceId"`
	Service         Service               `json:"service"`
	AssignedToID    *uuid.UUID            `gorm:"type:uuid;index" json:"assignedToId"`
	AssignedTo      *StaffUser            `gorm:"foreignKey:AssignedToID" json:"assignedTo,omitempty"`
	Status          RequestStatus         `gorm:"size:24;not null;default:new;index" json:"status"`
	Priority        int                   `gorm:"not null;default:0" json:"priority"`
	Quantity        int                   `gorm:"not null;default:1" json:"quantity"`
	Notes           string                `gorm:"size:500" json:"notes"`
	TotalPriceCents int64                 `gorm:"not null;default:0" json:"totalPriceCents"`
	StartedAt       *time.Time            `json:"startedAt"`
	CompletedAt     *time.Time            `json:"completedAt"`
	CancelledAt     *time.Time            `json:"cancelledAt"`
	Events          []ServiceRequestEvent `gorm:"foreignKey:RequestID" json:"events,omitempty"`
}

type RequestEventType string

const (
	RequestEventCreated  RequestEventType = "created"
	RequestEventAssigned RequestEventType = "assigned"
	RequestEventPriority RequestEventType = "priority_changed"
	RequestEventStatus   RequestEventType = "status_changed"
	RequestEventNote     RequestEventType = "note_added"
)

type ServiceRequestEvent struct {
	BaseModel
	HotelID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"hotelId"`
	RequestID  uuid.UUID        `gorm:"type:uuid;not null;index" json:"requestId"`
	Request    ServiceRequest   `json:"-"`
	EventType  RequestEventType `gorm:"size:32;not null;index" json:"eventType"`
	FromStatus *RequestStatus   `gorm:"size:24" json:"fromStatus,omitempty"`
	ToStatus   *RequestStatus   `gorm:"size:24" json:"toStatus,omitempty"`
	ActorType  string           `gorm:"size:16;not null" json:"actorType"`
	ActorID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"actorId"`
	Note       string           `gorm:"size:500" json:"note"`
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
