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
	IsPreArrival     bool            `gorm:"not null;default:false;index" json:"isPreArrival"`
	AvailableFrom    string          `gorm:"size:5" json:"availableFrom"`
	AvailableUntil   string          `gorm:"size:5" json:"availableUntil"`
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

func RevenueServices(hotelID uuid.UUID) []Service {
	return []Service{
		{HotelID: hotelID, Code: "spa-massage", Name: "اسپا و ماساژ", Description: "رزرو خدمات اسپا و ماساژ هتل", Category: ServiceCategoryWellness, Icon: "wellness", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 60, PriceCents: 8_500_000, Currency: "IRR", IsPaid: true, SortOrder: 110, IsActive: true},
		{HotelID: hotelID, Code: "minibar-package", Name: "بسته مینی‌بار", Description: "آماده‌سازی بسته کامل مینی‌بار در اتاق", Category: ServiceCategoryFNB, Icon: "coffee", FulfillmentRole: StaffRoleFB, EstimatedMinutes: 25, PriceCents: 3_200_000, Currency: "IRR", IsPaid: true, SortOrder: 120, IsActive: true},
		{HotelID: hotelID, Code: "tennis-court", Name: "زمین تنیس (۱ ساعت)", Description: "رزرو یک ساعت زمین تنیس", Category: ServiceCategoryWellness, Icon: "wellness", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 60, PriceCents: 4_500_000, Currency: "IRR", IsPaid: true, SortOrder: 130, IsActive: true},
		{HotelID: hotelID, Code: "prearrival-transfer", Name: "ترانسفر فرودگاه", Description: "استقبال فرودگاهی پیش از ورود", Category: ServiceCategoryTransport, Icon: "car", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 45, PriceCents: 6_800_000, Currency: "IRR", IsPaid: true, IsPreArrival: true, SortOrder: 140, IsActive: true},
		{HotelID: hotelID, Code: "prearrival-late-checkout", Name: "خروج دیرهنگام تا ۱۸", Description: "رزرو خروج دیرهنگام پیش از ورود", Category: ServiceCategoryOther, Icon: "clock", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 15, PriceCents: 9_000_000, Currency: "IRR", IsPaid: true, IsPreArrival: true, SortOrder: 150, IsActive: true},
		{HotelID: hotelID, Code: "welcome-flowers", Name: "گل و شیرینی در اتاق", Description: "گل و شیرینی آماده در لحظه ورود", Category: ServiceCategoryOther, Icon: "concierge", FulfillmentRole: StaffRoleReception, EstimatedMinutes: 30, PriceCents: 5_500_000, Currency: "IRR", IsPaid: true, IsPreArrival: true, SortOrder: 160, IsActive: true},
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
	Icon        string    `gorm:"size:32;not null;default:concierge" json:"icon"`
	Hours       string    `gorm:"size:120" json:"hours"`
	SortOrder   int       `gorm:"not null;default:0" json:"sortOrder"`
	IsActive    bool      `gorm:"not null;default:true" json:"isActive"`
}

type Promotion struct {
	BaseModel
	HotelID     uuid.UUID `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel     `json:"-"`
	Title       string    `gorm:"not null" json:"title"`
	Description string    `json:"description"`
	DiscountPct float64   `gorm:"not null;default:0" json:"discountPct"`
	BadgeText   string    `gorm:"size:32" json:"badgeText"`
	StartsAt    time.Time `gorm:"not null" json:"startsAt"`
	EndsAt      time.Time `gorm:"not null" json:"endsAt"`
	IsActive    bool      `gorm:"not null;default:true" json:"isActive"`
}

type Restaurant struct {
	BaseModel
	HotelID     uuid.UUID  `gorm:"not null;index" json:"hotelId"`
	Hotel       Hotel      `json:"-"`
	Name        string     `gorm:"not null" json:"name"`
	Description string     `json:"description"`
	Hours       string     `gorm:"size:120" json:"hours"`
	SortOrder   int        `gorm:"not null;default:0" json:"sortOrder"`
	IsActive    bool       `gorm:"not null;default:true" json:"isActive"`
	MenuItems   []MenuItem `gorm:"foreignKey:RestaurantID" json:"menuItems"`
}

type MenuItem struct {
	BaseModel
	RestaurantID uuid.UUID  `gorm:"not null;index" json:"restaurantId"`
	Restaurant   Restaurant `json:"-"`
	Name         string     `gorm:"not null" json:"name"`
	Description  string     `json:"description"`
	PriceCents   int64      `gorm:"not null;default:0" json:"priceCents"`
	Currency     string     `gorm:"size:3;not null;default:IRR" json:"currency"`
	SortOrder    int        `gorm:"not null;default:0" json:"sortOrder"`
	IsAvailable  bool       `gorm:"not null;default:true" json:"isAvailable"`
}

func DefaultFacilities(hotelID uuid.UUID) []Facility {
	return []Facility{
		{HotelID: hotelID, Name: "استخر و سونا", Description: "استخر سرپوشیده و سونای خشک", Icon: "water", Hours: "۷ تا ۲۳", SortOrder: 10, IsActive: true},
		{HotelID: hotelID, Name: "اسپا", Description: "ماساژ و خدمات آرامش", Icon: "wellness", Hours: "۱۰ تا ۲۲", SortOrder: 20, IsActive: true},
		{HotelID: hotelID, Name: "پارکینگ", Description: "پارکینگ اختصاصی مهمانان", Icon: "parking", Hours: "۲۴ ساعته", SortOrder: 30, IsActive: true},
		{HotelID: hotelID, Name: "رستوران", Description: "رستوران ایرانی و بین‌المللی", Icon: "restaurant", Hours: "۱۲ تا ۲۳", SortOrder: 40, IsActive: true},
		{HotelID: hotelID, Name: "سالن ورزش", Description: "تجهیزات هوازی و بدنسازی", Icon: "fitness", Hours: "۶ تا ۲۳", SortOrder: 50, IsActive: true},
		{HotelID: hotelID, Name: "وای‌فای رایگان", Description: "اینترنت پرسرعت در تمام هتل", Icon: "wifi", Hours: "۲۴ ساعته", SortOrder: 60, IsActive: true},
	}
}

func DefaultPromotion(hotelID uuid.UUID, now time.Time) Promotion {
	return Promotion{
		HotelID: hotelID, Title: "پیشنهاد ویژه اقامت", Description: "۱۵٪ تخفیف برای رزرو مستقیم سرویس‌های منتخب هتل",
		DiscountPct: 15, BadgeText: "٪۱۵", StartsAt: now.AddDate(0, -1, 0), EndsAt: now.AddDate(1, 0, 0), IsActive: true,
	}
}

func DefaultRestaurant(hotelID uuid.UUID) (Restaurant, []MenuItem) {
	restaurant := Restaurant{HotelID: hotelID, Name: "رستوران اصلی", Description: "غذاهای ایرانی و بین‌المللی", Hours: "سرو ۱۲ تا ۲۳", SortOrder: 10, IsActive: true}
	items := []MenuItem{
		{Name: "کباب سلطانی", Description: "راسته گوسفندی، برنج ایرانی و دورچین", PriceCents: 9_800_000, Currency: "IRR", SortOrder: 10, IsAvailable: true},
		{Name: "خوراک ماهی قزل‌آلا", Description: "ماهی تازه، سبزیجات و سس مخصوص", PriceCents: 7_200_000, Currency: "IRR", SortOrder: 20, IsAvailable: true},
		{Name: "پاستا آلفردو", Description: "مرغ، قارچ و سس خامه‌ای", PriceCents: 5_400_000, Currency: "IRR", SortOrder: 30, IsAvailable: true},
		{Name: "سوپ روز", Description: "تهیه‌شده با مواد تازه روز", PriceCents: 2_200_000, Currency: "IRR", SortOrder: 40, IsAvailable: true},
	}
	return restaurant, items
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
	HotelID       uuid.UUID       `gorm:"not null;index" json:"hotelId"`
	Hotel         Hotel           `json:"-"`
	Title         string          `gorm:"not null" json:"title"`
	Content       string          `gorm:"not null;type:text" json:"content"`
	Source        string          `gorm:"size:120;not null;default:staff" json:"source"`
	Status        KnowledgeStatus `gorm:"size:24;not null;default:draft;index" json:"status"`
	Version       int             `gorm:"not null;default:1" json:"version"`
	SupersedesID  *uuid.UUID      `gorm:"type:uuid;index" json:"supersedesId"`
	SubmittedByID *uuid.UUID      `gorm:"type:uuid;index" json:"submittedById"`
	ReviewedByID  *uuid.UUID      `gorm:"type:uuid;index" json:"reviewedById"`
	ReviewedAt    *time.Time      `json:"reviewedAt"`
	ReviewNote    string          `gorm:"size:500" json:"reviewNote"`
}

func DefaultKnowledge(hotelID uuid.UUID, now time.Time) []KnowledgeItem {
	items := []KnowledgeItem{
		{Title: "ساعت فعالیت استخر چیست؟", Content: "استخر سرپوشیده و سونا هر روز از ساعت ۷ تا ۲۳ در دسترس مهمانان است."},
		{Title: "صبحانه چه ساعتی سرو می‌شود؟", Content: "صبحانه هر روز از ساعت ۷ تا ۱۰:۳۰ در رستوران اصلی هتل سرو می‌شود."},
		{Title: "اطلاعات وای‌فای هتل چیست؟", Content: "وای‌فای پرسرعت در همه بخش‌های هتل رایگان است؛ اطلاعات اتصال روی کارت داخل اتاق قرار دارد."},
		{Title: "ساعت چک‌اوت چه زمانی است؟", Content: "زمان استاندارد خروج ساعت ۱۲ است. برای خروج دیرهنگام می‌توانید از کاتالوگ سرویس‌ها درخواست ثبت کنید."},
		{Title: "اسپا چه خدماتی دارد؟", Content: "اسپا هر روز از ساعت ۱۰ تا ۲۲ فعال است و رزرو ماساژ از بخش سرویس‌های ویژه انجام می‌شود."},
		{Title: "آیا هتل پارکینگ دارد؟", Content: "پارکینگ اختصاصی مهمانان به‌صورت ۲۴ ساعته و بدون هزینه اضافی در دسترس است."},
	}
	for index := range items {
		items[index].HotelID = hotelID
		items[index].Source = "محتوای تأییدشده هتل"
		items[index].Status = KnowledgeApproved
		items[index].Version = 1
		approvedAt := now
		items[index].ReviewedAt = &approvedAt
	}
	return items
}

type ConversationStatus string

const (
	ConversationAI        ConversationStatus = "ai"
	ConversationHandedOff ConversationStatus = "handed_off"
	ConversationClosed    ConversationStatus = "closed"
)

type Conversation struct {
	BaseModel
	HotelID       uuid.UUID          `gorm:"not null;index" json:"hotelId"`
	Hotel         Hotel              `json:"-"`
	GuestID       uuid.UUID          `gorm:"not null;index" json:"guestId"`
	Guest         Guest              `json:"guest"`
	StayID        *uuid.UUID         `gorm:"type:uuid;index;uniqueIndex" json:"stayId"`
	Stay          *Stay              `json:"stay,omitempty"`
	AssignedToID  *uuid.UUID         `gorm:"type:uuid;index" json:"assignedToId"`
	AssignedTo    *StaffUser         `gorm:"foreignKey:AssignedToID" json:"assignedTo,omitempty"`
	Status        ConversationStatus `gorm:"size:24;not null;default:ai;index" json:"status"`
	GuestReadAt   *time.Time         `json:"guestReadAt"`
	StaffReadAt   *time.Time         `json:"staffReadAt"`
	LastMessageAt time.Time          `gorm:"not null;index" json:"lastMessageAt"`
	Messages      []Message          `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
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
	ConversationID  uuid.UUID    `gorm:"not null;index" json:"conversationId"`
	Conversation    Conversation `json:"-"`
	Role            MessageRole  `gorm:"size:16;not null" json:"role"`
	SenderID        *uuid.UUID   `gorm:"type:uuid;index" json:"senderId"`
	KnowledgeItemID *uuid.UUID   `gorm:"type:uuid;index" json:"knowledgeItemId"`
	Body            string       `gorm:"not null;type:text" json:"body"`
	Confidence      *float64     `json:"confidence,omitempty"`
	Redacted        bool         `gorm:"not null;default:false" json:"redacted"`
	ExpiresAt       time.Time    `gorm:"not null;index" json:"expiresAt"`
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
	RequestID string          `gorm:"size:128;index" json:"requestId"`
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
