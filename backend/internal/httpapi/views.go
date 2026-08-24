package httpapi

import (
	"encoding/json"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

type hotelView struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	LogoURL      string    `json:"logoUrl"`
	PrimaryColor string    `json:"primaryColor"`
	Timezone     string    `json:"timezone"`
}

func toHotelView(hotel models.Hotel) hotelView {
	return hotelView{
		ID: hotel.ID, Name: hotel.Name, Slug: hotel.Slug, LogoURL: hotel.LogoURL,
		PrimaryColor: hotel.PrimaryColor, Timezone: hotel.Timezone,
	}
}

type staffView struct {
	ID          uuid.UUID        `json:"id"`
	FirstName   string           `json:"firstName"`
	LastName    string           `json:"lastName"`
	Email       string           `json:"email"`
	Role        models.StaffRole `json:"role"`
	IsActive    bool             `json:"isActive"`
	LastLoginAt *time.Time       `json:"lastLoginAt"`
}

func toStaffView(staff models.StaffUser) staffView {
	return staffView{
		ID: staff.ID, FirstName: staff.FirstName, LastName: staff.LastName,
		Email: staff.Email, Role: staff.Role, IsActive: staff.IsActive, LastLoginAt: staff.LastLoginAt,
	}
}

type guestView struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
}

type roomView struct {
	ID     uuid.UUID         `json:"id"`
	Number string            `json:"number"`
	Floor  int               `json:"floor"`
	Type   string            `json:"type"`
	Status models.RoomStatus `json:"status"`
}

func toRoomView(room models.Room) roomView {
	return roomView{ID: room.ID, Number: room.Number, Floor: room.Floor, Type: room.Type, Status: room.Status}
}

type reservationView struct {
	ID               uuid.UUID                `json:"id"`
	Status           models.ReservationStatus `json:"status"`
	ConfirmationCode string                   `json:"confirmationCode"`
	ArrivalDate      time.Time                `json:"arrivalDate"`
	DepartureDate    time.Time                `json:"departureDate"`
	ConfirmedAt      *time.Time               `json:"confirmedAt"`
	Guest            guestView                `json:"guest"`
	Room             *roomView                `json:"room"`
	Stay             *staySummaryView         `json:"stay,omitempty"`
}

type staySummaryView struct {
	ID         uuid.UUID         `json:"id"`
	Status     models.StayStatus `json:"status"`
	Room       roomView          `json:"room"`
	CheckInAt  *time.Time        `json:"checkInAt"`
	CheckOutAt *time.Time        `json:"checkOutAt"`
}

func toReservationView(reservation models.Reservation) reservationView {
	view := reservationView{
		ID: reservation.ID, Status: reservation.Status, ConfirmationCode: reservation.ConfirmationCode,
		ArrivalDate: reservation.ArrivalDate, DepartureDate: reservation.DepartureDate,
		ConfirmedAt: reservation.ConfirmedAt,
		Guest:       guestView{ID: reservation.Guest.ID, FirstName: reservation.Guest.FirstName, LastName: reservation.Guest.LastName},
	}
	if reservation.Room != nil {
		room := toRoomView(*reservation.Room)
		view.Room = &room
	}
	if reservation.Stay != nil {
		view.Stay = &staySummaryView{ID: reservation.Stay.ID, Status: reservation.Stay.Status, Room: toRoomView(reservation.Stay.Room), CheckInAt: reservation.Stay.CheckInAt, CheckOutAt: reservation.Stay.CheckOutAt}
	}
	return view
}

type stayView struct {
	ID          uuid.UUID         `json:"id"`
	Status      models.StayStatus `json:"status"`
	Guest       guestView         `json:"guest"`
	Room        roomView          `json:"room"`
	Hotel       hotelView         `json:"hotel"`
	CheckIn     *time.Time        `json:"checkInAt"`
	CheckOut    *time.Time        `json:"checkOutAt"`
	Reservation *reservationView  `json:"reservation,omitempty"`
}

func toStayView(stay models.Stay) stayView {
	view := stayView{
		ID: stay.ID, Status: stay.Status,
		Guest: guestView{ID: stay.Guest.ID, FirstName: stay.Guest.FirstName, LastName: stay.Guest.LastName},
		Room:  toRoomView(stay.Room),
		Hotel: toHotelView(stay.Hotel), CheckIn: stay.CheckInAt, CheckOut: stay.CheckOutAt,
	}
	if stay.Reservation != nil {
		reservation := toReservationView(*stay.Reservation)
		view.Reservation = &reservation
	}
	return view
}

type onlineCheckInView struct {
	ID                uuid.UUID                  `json:"id"`
	Status            models.OnlineCheckInStatus `json:"status"`
	DocumentName      string                     `json:"documentName"`
	DocumentMediaType string                     `json:"documentMediaType"`
	DocumentSize      int64                      `json:"documentSize"`
	SubmittedAt       time.Time                  `json:"submittedAt"`
	ReviewedAt        *time.Time                 `json:"reviewedAt"`
	ReviewedByID      *uuid.UUID                 `json:"reviewedById"`
	ReviewNote        string                     `json:"reviewNote"`
	RetentionUntil    time.Time                  `json:"retentionUntil"`
	DocumentAvailable bool                       `json:"documentAvailable"`
	Stay              *stayView                  `json:"stay,omitempty"`
}

func toOnlineCheckInView(checkIn models.OnlineCheckIn, includeStay bool) onlineCheckInView {
	view := onlineCheckInView{
		ID: checkIn.ID, Status: checkIn.Status, DocumentName: checkIn.DocumentName,
		DocumentMediaType: checkIn.DocumentMediaType, DocumentSize: checkIn.DocumentSize,
		SubmittedAt: checkIn.SubmittedAt, ReviewedAt: checkIn.ReviewedAt,
		ReviewedByID: checkIn.ReviewedByID, ReviewNote: checkIn.ReviewNote,
		RetentionUntil: checkIn.RetentionUntil, DocumentAvailable: checkIn.DocumentDeletedAt == nil,
	}
	if includeStay {
		stay := toStayView(checkIn.Stay)
		view.Stay = &stay
	}
	return view
}

type arrivalSettingsView struct {
	OnlineCheckInEnabled       bool            `json:"onlineCheckInEnabled"`
	DigitalRegistrationEnabled bool            `json:"digitalRegistrationEnabled"`
	PaymentStepEnabled         bool            `json:"paymentStepEnabled"`
	InvitationTTLHours         int             `json:"invitationTtlHours"`
	TermsVersion               string          `json:"termsVersion"`
	TermsLocale                string          `json:"termsLocale"`
	TermsText                  string          `json:"termsText"`
	Steps                      json.RawMessage `json:"steps"`
	DocumentPurpose            string          `json:"documentPurpose"`
}

func toArrivalSettingsView(settings models.ArrivalSettings) arrivalSettingsView {
	return arrivalSettingsView{
		OnlineCheckInEnabled: settings.OnlineCheckInEnabled, DigitalRegistrationEnabled: settings.DigitalRegistrationEnabled,
		PaymentStepEnabled: settings.PaymentStepEnabled, InvitationTTLHours: settings.InvitationTTLHours,
		TermsVersion: settings.TermsVersion, TermsLocale: settings.TermsLocale, TermsText: settings.TermsText, Steps: settings.StepsJSON,
		DocumentPurpose: "مدارک فقط برای تطبیق هویت و تکمیل ثبت اقامت در اختیار پرسنل مجاز قرار می‌گیرد و پس از پایان دوره نگهداری حذف می‌شود.",
	}
}

type arrivalDocumentView struct {
	ID                uuid.UUID  `json:"id"`
	CompanionID       *uuid.UUID `json:"companionId"`
	EvidenceType      string     `json:"evidenceType"`
	Side              string     `json:"side"`
	Name              string     `json:"name"`
	MediaType         string     `json:"mediaType"`
	Size              int64      `json:"size"`
	VerificationState string     `json:"verificationState"`
	VerificationNote  string     `json:"verificationNote"`
	RetentionUntil    time.Time  `json:"retentionUntil"`
	CreatedAt         time.Time  `json:"createdAt"`
}

func toArrivalDocumentView(document models.ArrivalDocument) arrivalDocumentView {
	return arrivalDocumentView{
		ID: document.ID, CompanionID: document.CompanionID, EvidenceType: document.EvidenceType, Side: document.Side,
		Name: document.Name, MediaType: document.MediaType, Size: document.Size,
		VerificationState: document.VerificationState, VerificationNote: document.VerificationNote,
		RetentionUntil: document.RetentionUntil, CreatedAt: document.CreatedAt,
	}
}

type arrivalCompanionView struct {
	ID               uuid.UUID  `json:"id"`
	FirstName        string     `json:"firstName"`
	LastName         string     `json:"lastName"`
	Relationship     string     `json:"relationship"`
	Nationality      string     `json:"nationality"`
	DateOfBirth      *time.Time `json:"dateOfBirth"`
	DocumentRequired bool       `json:"documentRequired"`
}

type arrivalPaymentStepView struct {
	Required   bool   `json:"required"`
	Status     string `json:"status"`
	Capability string `json:"capability"`
}

type arrivalView struct {
	ID                 uuid.UUID               `json:"id"`
	Status             models.ArrivalStatus    `json:"status"`
	CurrentStep        int                     `json:"currentStep"`
	CompletenessScore  int                     `json:"completenessScore"`
	RiskState          string                  `json:"riskState"`
	ContactPhone       string                  `json:"contactPhone"`
	ContactEmail       string                  `json:"contactEmail"`
	Nationality        string                  `json:"nationality"`
	ArrivalETA         *time.Time              `json:"arrivalEta"`
	ArrivalMethod      string                  `json:"arrivalMethod"`
	TransportDetails   string                  `json:"transportDetails"`
	AccessibilityNeeds string                  `json:"accessibilityNeeds"`
	SpecialRequests    string                  `json:"specialRequests"`
	Answers            json.RawMessage         `json:"answers"`
	TermsVersion       string                  `json:"termsVersion"`
	TermsLocale        string                  `json:"termsLocale"`
	SignerName         string                  `json:"signerName"`
	ConsentAt          *time.Time              `json:"consentAt"`
	SignaturePresent   bool                    `json:"signaturePresent"`
	SubmittedAt        *time.Time              `json:"submittedAt"`
	ReviewedAt         *time.Time              `json:"reviewedAt"`
	ReviewedByID       *uuid.UUID              `json:"reviewedById"`
	ReviewOwnerID      *uuid.UUID              `json:"reviewOwnerId"`
	NeedsChangesReason string                  `json:"needsChangesReason"`
	ApprovedAt         *time.Time              `json:"approvedAt"`
	ArrivalPendingAt   *time.Time              `json:"arrivalPendingAt"`
	RoomReadyAt        *time.Time              `json:"roomReadyAt"`
	CheckedInAt        *time.Time              `json:"checkedInAt"`
	CancelledAt        *time.Time              `json:"cancelledAt"`
	ExpiresAt          time.Time               `json:"expiresAt"`
	Reservation        reservationView         `json:"reservation"`
	Stay               stayView                `json:"stay"`
	Documents          []arrivalDocumentView   `json:"documents"`
	Companions         []arrivalCompanionView  `json:"companions"`
	PaymentStep        *arrivalPaymentStepView `json:"paymentStep,omitempty"`
}

func toArrivalView(journey models.ArrivalJourney) arrivalView {
	view := arrivalView{
		ID: journey.ID, Status: journey.Status, CurrentStep: journey.CurrentStep, CompletenessScore: journey.CompletenessScore,
		RiskState: journey.RiskState, ContactPhone: journey.ContactPhone, ContactEmail: journey.ContactEmail,
		Nationality: journey.Nationality, ArrivalETA: journey.ArrivalETA, ArrivalMethod: journey.ArrivalMethod,
		TransportDetails: journey.TransportDetails, AccessibilityNeeds: journey.AccessibilityNeeds, SpecialRequests: journey.SpecialRequests,
		Answers: journey.AnswersJSON, TermsVersion: journey.TermsVersion, TermsLocale: journey.TermsLocale,
		SignerName: journey.SignerName, ConsentAt: journey.ConsentAt,
		SignaturePresent: journey.SignatureStorageKey != "" && journey.SignatureDeletedAt == nil,
		SubmittedAt:      journey.SubmittedAt, ReviewedAt: journey.ReviewedAt, ReviewedByID: journey.ReviewedByID,
		ReviewOwnerID: journey.ReviewOwnerID, NeedsChangesReason: journey.NeedsChangesReason,
		ApprovedAt: journey.ApprovedAt, ArrivalPendingAt: journey.ArrivalPendingAt, RoomReadyAt: journey.RoomReadyAt,
		CheckedInAt: journey.CheckedInAt, CancelledAt: journey.CancelledAt, ExpiresAt: journey.ExpiresAt,
		Reservation: toReservationView(journey.Reservation), Stay: toStayView(journey.Stay),
		Documents: make([]arrivalDocumentView, 0, len(journey.Documents)), Companions: make([]arrivalCompanionView, 0, len(journey.Companions)),
	}
	if journey.PaymentStep != nil {
		view.PaymentStep = &arrivalPaymentStepView{Required: journey.PaymentStep.Required, Status: journey.PaymentStep.Status, Capability: journey.PaymentStep.Capability}
	}
	if len(view.Answers) == 0 {
		view.Answers = json.RawMessage(`{}`)
	}
	for _, document := range journey.Documents {
		view.Documents = append(view.Documents, toArrivalDocumentView(document))
	}
	for _, companion := range journey.Companions {
		view.Companions = append(view.Companions, arrivalCompanionView{ID: companion.ID, FirstName: companion.FirstName, LastName: companion.LastName, Relationship: companion.Relationship, Nationality: companion.Nationality, DateOfBirth: companion.DateOfBirth, DocumentRequired: companion.DocumentRequired})
	}
	return view
}

type serviceView struct {
	ID               uuid.UUID              `json:"id"`
	Code             string                 `json:"code"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Category         models.ServiceCategory `json:"category"`
	Icon             string                 `json:"icon"`
	FulfillmentRole  models.StaffRole       `json:"fulfillmentRole"`
	EstimatedMinutes int                    `json:"estimatedMinutes"`
	PriceCents       int64                  `json:"priceCents"`
	Currency         string                 `json:"currency"`
	IsPaid           bool                   `json:"isPaid"`
	IsQuickAction    bool                   `json:"isQuickAction"`
	IsPreArrival     bool                   `json:"isPreArrival"`
	AvailableFrom    string                 `json:"availableFrom"`
	AvailableUntil   string                 `json:"availableUntil"`
	SortOrder        int                    `json:"sortOrder"`
	IsActive         bool                   `json:"isActive"`
}

func toServiceView(service models.Service) serviceView {
	return serviceView{
		ID: service.ID, Code: service.Code, Name: service.Name, Description: service.Description,
		Category: service.Category, Icon: service.Icon, FulfillmentRole: service.FulfillmentRole,
		EstimatedMinutes: service.EstimatedMinutes, PriceCents: service.PriceCents, Currency: service.Currency,
		IsPaid: service.IsPaid, IsQuickAction: service.IsQuickAction, SortOrder: service.SortOrder, IsActive: service.IsActive,
		IsPreArrival: service.IsPreArrival, AvailableFrom: service.AvailableFrom, AvailableUntil: service.AvailableUntil,
	}
}

type requestEventView struct {
	ID         uuid.UUID               `json:"id"`
	EventType  models.RequestEventType `json:"eventType"`
	FromStatus *models.RequestStatus   `json:"fromStatus,omitempty"`
	ToStatus   *models.RequestStatus   `json:"toStatus,omitempty"`
	ActorType  string                  `json:"actorType"`
	Note       string                  `json:"note"`
	CreatedAt  time.Time               `json:"createdAt"`
}

type serviceRequestView struct {
	ID              uuid.UUID            `json:"id"`
	Status          models.RequestStatus `json:"status"`
	Priority        int                  `json:"priority"`
	Quantity        int                  `json:"quantity"`
	Notes           string               `json:"notes"`
	TotalPriceCents int64                `json:"totalPriceCents"`
	Service         serviceView          `json:"service"`
	AssignedTo      *staffView           `json:"assignedTo"`
	StartedAt       *time.Time           `json:"startedAt"`
	CompletedAt     *time.Time           `json:"completedAt"`
	CancelledAt     *time.Time           `json:"cancelledAt"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	Events          []requestEventView   `json:"events"`
	Stay            *requestStayView     `json:"stay,omitempty"`
}

type requestStayView struct {
	ID    uuid.UUID `json:"id"`
	Guest guestView `json:"guest"`
	Room  roomView  `json:"room"`
}

func toServiceRequestView(request models.ServiceRequest, includeStay bool) serviceRequestView {
	view := serviceRequestView{
		ID: request.ID, Status: request.Status, Priority: request.Priority, Quantity: request.Quantity,
		Notes: request.Notes, TotalPriceCents: request.TotalPriceCents, Service: toServiceView(request.Service),
		StartedAt: request.StartedAt, CompletedAt: request.CompletedAt, CancelledAt: request.CancelledAt,
		CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt,
		Events: make([]requestEventView, 0, len(request.Events)),
	}
	if request.AssignedTo != nil {
		staff := toStaffView(*request.AssignedTo)
		view.AssignedTo = &staff
	}
	for _, event := range request.Events {
		view.Events = append(view.Events, requestEventView{
			ID: event.ID, EventType: event.EventType, FromStatus: event.FromStatus, ToStatus: event.ToStatus,
			ActorType: event.ActorType, Note: event.Note, CreatedAt: event.CreatedAt,
		})
	}
	if includeStay {
		view.Stay = &requestStayView{
			ID:    request.Stay.ID,
			Guest: guestView{ID: request.Stay.Guest.ID, FirstName: request.Stay.Guest.FirstName, LastName: request.Stay.Guest.LastName},
			Room:  toRoomView(request.Stay.Room),
		}
	}
	return view
}
