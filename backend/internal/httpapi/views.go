package httpapi

import (
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
