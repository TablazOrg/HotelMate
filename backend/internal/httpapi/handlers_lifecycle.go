package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var lifecycleRoles = []models.StaffRole{
	models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations, models.StaffRoleReception,
}

func (s *Server) registerLifecycleRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/staff/rooms", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.listRooms)))
	mux.Handle("POST /api/v1/staff/rooms", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.createRoom)))
	mux.Handle("PATCH /api/v1/staff/rooms/{id}/status", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.updateRoomStatus)))
	mux.Handle("GET /api/v1/staff/reservations", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.listReservations)))
	mux.Handle("POST /api/v1/staff/reservations", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.createReservation)))
	mux.Handle("POST /api/v1/staff/reservations/{id}/confirm", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.confirmReservation)))
	mux.Handle("POST /api/v1/staff/stays/{id}/check-in", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.checkInStay)))
	mux.Handle("POST /api/v1/staff/stays/{id}/check-out", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.checkOutStay)))
	mux.Handle("GET /api/v1/guest/online-check-in", s.require(auth.ActorGuest)(http.HandlerFunc(s.guestOnlineCheckIn)))
	mux.Handle("POST /api/v1/guest/online-check-in", s.require(auth.ActorGuest)(http.HandlerFunc(s.submitOnlineCheckIn)))
	mux.Handle("GET /api/v1/staff/online-check-ins", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.listOnlineCheckIns)))
	mux.Handle("POST /api/v1/staff/online-check-ins/{id}/review", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.reviewOnlineCheckIn)))
	mux.Handle("GET /api/v1/staff/online-check-ins/{id}/document", s.require(auth.ActorStaff, lifecycleRoles...)(http.HandlerFunc(s.downloadOnlineCheckInDocument)))
}

func (s *Server) requireLifecycle(w http.ResponseWriter) bool {
	if s.lifecycle == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس اقامت در دسترس نیست")
		return false
	}
	return true
}

type createRoomRequest struct {
	Number string `json:"number"`
	Floor  int    `json:"floor"`
	Type   string `json:"type"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input createRoomRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Number = normalizeRoom(input.Number)
	input.Type = strings.TrimSpace(input.Type)
	if input.Number == "" || len(input.Number) > 20 || input.Floor < -5 || input.Floor > 200 || len(input.Type) > 64 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات اتاق معتبر نیست")
		return
	}
	room := models.Room{HotelID: staff.HotelID, Number: input.Number, Floor: input.Floor, Type: input.Type, Status: models.RoomStatusAvailable}
	if err := s.lifecycle.CreateRoom(r.Context(), &room); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "room_exists", "شماره اتاق قبلاً ثبت شده است")
			return
		}
		s.logger.Error("create room", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد اتاق انجام نشد")
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "room.create", models.AuditOutcomeSuccess, map[string]any{"roomId": room.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"room": toRoomView(room)})
}

func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	rooms, err := s.lifecycle.ListRooms(r.Context(), staff.HotelID)
	if err != nil {
		s.logger.Error("list rooms", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت اتاق‌ها انجام نشد")
		return
	}
	views := make([]roomView, 0, len(rooms))
	for _, room := range rooms {
		views = append(views, toRoomView(room))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": views})
}

type roomStatusRequest struct {
	Status models.RoomStatus `json:"status"`
}

func (s *Server) updateRoomStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_room", "شناسه اتاق معتبر نیست")
		return
	}
	var input roomStatusRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if !validRoomStatus(input.Status) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "وضعیت اتاق معتبر نیست")
		return
	}
	room, err := s.lifecycle.UpdateRoomStatus(r.Context(), staff.HotelID, roomID, input.Status)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "room.status.update", models.AuditOutcomeSuccess, map[string]any{"roomId": room.ID, "status": room.Status})
	writeJSON(w, http.StatusOK, map[string]any{"room": toRoomView(room)})
}

type createReservationRequest struct {
	Guest struct {
		FirstName      string `json:"firstName"`
		LastName       string `json:"lastName"`
		IdentityType   string `json:"identityType"`
		IdentityNumber string `json:"identityNumber"`
		Phone          string `json:"phone"`
	} `json:"guest"`
	RoomID        uuid.UUID `json:"roomId"`
	ArrivalDate   string    `json:"arrivalDate"`
	DepartureDate string    `json:"departureDate"`
}

func (s *Server) createReservation(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input createReservationRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Guest.FirstName = strings.TrimSpace(input.Guest.FirstName)
	input.Guest.LastName = strings.TrimSpace(input.Guest.LastName)
	input.Guest.IdentityType = strings.ToLower(strings.TrimSpace(input.Guest.IdentityType))
	input.Guest.Phone = strings.TrimSpace(input.Guest.Phone)
	arrival, errA := parseReservationDate(input.ArrivalDate)
	departure, errD := parseReservationDate(input.DepartureDate)
	if len(input.Guest.FirstName) < 2 || len(input.Guest.LastName) < 2 || (input.Guest.IdentityType != "national_id" && input.Guest.IdentityType != "passport") || input.RoomID == uuid.Nil || errA != nil || errD != nil || !departure.After(arrival) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات رزرو معتبر نیست")
		return
	}
	identityHash, err := auth.HashIdentity(input.Guest.IdentityNumber)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "مدرک هویتی معتبر نیست")
		return
	}
	code, err := confirmationCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد کد رزرو انجام نشد")
		return
	}
	guest := models.Guest{FirstName: input.Guest.FirstName, LastName: input.Guest.LastName, IdentityType: input.Guest.IdentityType, IdentityNumberHash: identityHash, Phone: input.Guest.Phone}
	reservation := models.Reservation{HotelID: staff.HotelID, RoomID: &input.RoomID, ConfirmationCode: code, Status: models.ReservationPending, ArrivalDate: arrival, DepartureDate: departure}
	if err := s.lifecycle.CreateReservation(r.Context(), &guest, &reservation); err != nil {
		if errors.Is(err, store.ErrReservationOverlap) {
			writeError(w, http.StatusConflict, "reservation_overlap", "این اتاق در بازه انتخابی رزرو شده است")
			return
		}
		if errors.Is(err, store.ErrRoomUnavailable) || errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusConflict, "room_unavailable", "اتاق انتخابی در دسترس نیست")
			return
		}
		s.logger.Error("create reservation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد رزرو انجام نشد")
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "reservation.create", models.AuditOutcomeSuccess, map[string]any{"reservationId": reservation.ID, "roomId": input.RoomID})
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": toReservationView(reservation)})
}

func (s *Server) listReservations(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	status := models.ReservationStatus(r.URL.Query().Get("status"))
	if !validReservationStatus(status) {
		writeError(w, http.StatusBadRequest, "invalid_filter", "فیلتر وضعیت معتبر نیست")
		return
	}
	filter := store.ReservationFilter{Status: status}
	if value := r.URL.Query().Get("from"); value != "" {
		parsed, err := parseReservationDate(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "تاریخ شروع معتبر نیست")
			return
		}
		filter.From = &parsed
	}
	if value := r.URL.Query().Get("to"); value != "" {
		parsed, err := parseReservationDate(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "تاریخ پایان معتبر نیست")
			return
		}
		filter.To = &parsed
	}
	reservations, err := s.lifecycle.ListReservations(r.Context(), staff.HotelID, filter)
	if err != nil {
		s.logger.Error("list reservations", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت رزروها انجام نشد")
		return
	}
	views := make([]reservationView, 0, len(reservations))
	for _, reservation := range reservations {
		views = append(views, toReservationView(reservation))
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservations": views})
}

func (s *Server) confirmReservation(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	reservationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_reservation", "شناسه رزرو معتبر نیست")
		return
	}
	reservation, stay, err := s.lifecycle.ConfirmReservation(r.Context(), staff.HotelID, reservationID, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "reservation.confirm", models.AuditOutcomeSuccess, map[string]any{"reservationId": reservation.ID, "stayId": stay.ID})
	writeJSON(w, http.StatusOK, map[string]any{"reservation": toReservationView(reservation), "stay": toStayView(stay)})
}

type checkInStayRequest struct {
	RoomID uuid.UUID `json:"roomId"`
}

func (s *Server) checkInStay(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	stayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_stay", "شناسه اقامت معتبر نیست")
		return
	}
	var input checkInStayRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.RoomID == uuid.Nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اتاق الزامی است")
		return
	}
	stay, err := s.lifecycle.CheckInStay(r.Context(), staff.HotelID, stayID, input.RoomID, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "stay.check_in", models.AuditOutcomeSuccess, map[string]any{"stayId": stay.ID, "roomId": stay.RoomID})
	writeJSON(w, http.StatusOK, map[string]any{"stay": toStayView(stay)})
}

func (s *Server) checkOutStay(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	stayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_stay", "شناسه اقامت معتبر نیست")
		return
	}
	stay, err := s.lifecycle.CheckOutStay(r.Context(), staff.HotelID, stayID, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "stay.check_out", models.AuditOutcomeSuccess, map[string]any{"stayId": stay.ID, "roomId": stay.RoomID})
	writeJSON(w, http.StatusOK, map[string]any{"stay": toStayView(stay)})
}

func parseReservationDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Parse("2006-01-02", value)
}

func confirmationCode() (string, error) {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buffer)), nil
}

func writeLifecycleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "not_found", "مورد درخواستی پیدا نشد")
	case errors.Is(err, store.ErrRoomUnavailable):
		writeError(w, http.StatusConflict, "room_unavailable", "اتاق در دسترس نیست")
	case errors.Is(err, store.ErrReservationOverlap):
		writeError(w, http.StatusConflict, "reservation_overlap", "رزرو با بازه دیگری تداخل دارد")
	case errors.Is(err, store.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_transition", "تغییر وضعیت در شرایط فعلی مجاز نیست")
	default:
		writeError(w, http.StatusInternalServerError, "internal_server_error", "عملیات انجام نشد")
	}
}
