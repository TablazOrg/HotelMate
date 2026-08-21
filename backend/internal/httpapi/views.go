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
	ID     uuid.UUID `json:"id"`
	Number string    `json:"number"`
	Floor  int       `json:"floor"`
	Type   string    `json:"type"`
}

type stayView struct {
	ID       uuid.UUID         `json:"id"`
	Status   models.StayStatus `json:"status"`
	Guest    guestView         `json:"guest"`
	Room     roomView          `json:"room"`
	Hotel    hotelView         `json:"hotel"`
	CheckIn  *time.Time        `json:"checkInAt"`
	CheckOut *time.Time        `json:"checkOutAt"`
}

func toStayView(stay models.Stay) stayView {
	return stayView{
		ID: stay.ID, Status: stay.Status,
		Guest: guestView{ID: stay.Guest.ID, FirstName: stay.Guest.FirstName, LastName: stay.Guest.LastName},
		Room:  roomView{ID: stay.Room.ID, Number: stay.Room.Number, Floor: stay.Room.Floor, Type: stay.Room.Type},
		Hotel: toHotelView(stay.Hotel), CheckIn: stay.CheckInAt, CheckOut: stay.CheckOutAt,
	}
}
