package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
)

type staffLoginRequest struct {
	HotelSlug string `json:"hotelSlug"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (s *Server) staffLogin(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || s.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس ورود در دسترس نیست")
		return
	}
	var input staffLoginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.HotelSlug = normalizeSlug(input.HotelSlug)
	input.Email = normalizeEmail(input.Email)
	if !slugPattern.MatchString(input.HotelSlug) || !validEmail(input.Email) || input.Password == "" {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}

	staff, err := s.store.FindStaffForLogin(r.Context(), input.HotelSlug, input.Email)
	if err != nil {
		auth.VerifyPassword(s.dummyHash, input.Password)
		s.audit(r, nil, nil, "staff", "auth.staff.login", models.AuditOutcomeFailure, map[string]any{"hotelSlug": input.HotelSlug})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}
	if !staff.IsActive || !auth.VerifyPassword(staff.PasswordHash, input.Password) {
		s.audit(r, &staff.HotelID, &staff.ID, "staff", "auth.staff.login", models.AuditOutcomeFailure, map[string]any{"hotelSlug": input.HotelSlug})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}
	token, expiresAt, err := s.tokens.IssueStaff(staff)
	if err != nil {
		s.logger.Error("issue staff token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد نشست انجام نشد")
		return
	}
	now := time.Now().UTC()
	staff.LastLoginAt = &now
	if err := s.store.MarkStaffLogin(r.Context(), staff.HotelID, staff.ID, now); err != nil {
		s.logger.Error("mark staff login", "staffId", staff.ID, "error", err)
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "auth.staff.login", models.AuditOutcomeSuccess, map[string]any{"role": staff.Role})
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token, "tokenType": "Bearer", "expiresAt": expiresAt,
		"actorType": "staff", "staff": toStaffView(staff), "hotel": toHotelView(staff.Hotel),
	})
}

type guestLoginRequest struct {
	HotelSlug      string `json:"hotelSlug"`
	RoomNumber     string `json:"roomNumber"`
	IdentityNumber string `json:"identityNumber"`
}

func (s *Server) guestLogin(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || s.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس ورود در دسترس نیست")
		return
	}
	var input guestLoginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.HotelSlug = normalizeSlug(input.HotelSlug)
	input.RoomNumber = normalizeRoom(input.RoomNumber)
	input.IdentityNumber = strings.TrimSpace(input.IdentityNumber)
	if !slugPattern.MatchString(input.HotelSlug) || input.RoomNumber == "" || len(auth.NormalizeIdentity(input.IdentityNumber)) < 4 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}

	stay, err := s.store.FindActiveStayForGuestLogin(r.Context(), input.HotelSlug, input.RoomNumber)
	if err != nil {
		auth.VerifyIdentity(s.dummyHash, input.IdentityNumber)
		s.audit(r, nil, nil, "guest", "auth.guest.login", models.AuditOutcomeFailure, map[string]any{"hotelSlug": input.HotelSlug, "roomNumber": input.RoomNumber})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}
	if stay.Guest.HotelID != stay.HotelID || !auth.VerifyIdentity(stay.Guest.IdentityNumberHash, input.IdentityNumber) {
		s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "auth.guest.login", models.AuditOutcomeFailure, map[string]any{"roomNumber": input.RoomNumber})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات ورود صحیح نیست")
		return
	}
	token, expiresAt, err := s.tokens.IssueGuest(stay)
	if err != nil {
		s.logger.Error("issue guest token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد نشست انجام نشد")
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "auth.guest.login", models.AuditOutcomeSuccess, map[string]any{"roomNumber": stay.Room.Number, "stayId": stay.ID})
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token, "tokenType": "Bearer", "expiresAt": expiresAt,
		"actorType": "guest", "stay": toStayView(stay),
	})
}

type reservationLoginRequest struct {
	HotelSlug        string `json:"hotelSlug"`
	ConfirmationCode string `json:"confirmationCode"`
	IdentityNumber   string `json:"identityNumber"`
}

func (s *Server) reservationLogin(w http.ResponseWriter, r *http.Request) {
	if s.lifecycle == nil || s.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس ورود در دسترس نیست")
		return
	}
	var input reservationLoginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.HotelSlug = normalizeSlug(input.HotelSlug)
	input.ConfirmationCode = strings.ToUpper(strings.TrimSpace(input.ConfirmationCode))
	input.IdentityNumber = strings.TrimSpace(input.IdentityNumber)
	if !slugPattern.MatchString(input.HotelSlug) || len(input.ConfirmationCode) < 6 || len(auth.NormalizeIdentity(input.IdentityNumber)) < 4 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات رزرو صحیح نیست")
		return
	}
	reservation, err := s.lifecycle.FindReservationForGuestLogin(r.Context(), input.HotelSlug, input.ConfirmationCode)
	if err != nil {
		auth.VerifyIdentity(s.dummyHash, input.IdentityNumber)
		s.audit(r, nil, nil, "guest", "auth.reservation.login", models.AuditOutcomeFailure, map[string]any{"hotelSlug": input.HotelSlug})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات رزرو صحیح نیست")
		return
	}
	if reservation.Guest.HotelID != reservation.HotelID || !auth.VerifyIdentity(reservation.Guest.IdentityNumberHash, input.IdentityNumber) {
		s.audit(r, &reservation.HotelID, &reservation.GuestID, "guest", "auth.reservation.login", models.AuditOutcomeFailure, map[string]any{})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "اطلاعات رزرو صحیح نیست")
		return
	}
	stay, err := s.lifecycle.EnsurePreArrivalStay(r.Context(), reservation.HotelID, reservation.ID)
	if err != nil {
		s.logger.Error("ensure pre-arrival stay", "reservationId", reservation.ID, "error", err)
		writeError(w, http.StatusConflict, "reservation_unavailable", "رزرو برای ورود پیش از اقامت آماده نیست")
		return
	}
	token, expiresAt, err := s.tokens.IssueGuest(stay)
	if err != nil {
		s.logger.Error("issue pre-arrival token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد نشست انجام نشد")
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "auth.reservation.login", models.AuditOutcomeSuccess, map[string]any{"reservationId": reservation.ID, "stayId": stay.ID})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "tokenType": "Bearer", "expiresAt": expiresAt, "actorType": "guest", "stay": toStayView(stay)})
}

func (s *Server) staffMe(w http.ResponseWriter, r *http.Request) {
	staff, ok := currentStaff(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actorType": "staff", "staff": toStaffView(staff), "hotel": toHotelView(staff.Hotel)})
}

func (s *Server) guestMe(w http.ResponseWriter, r *http.Request) {
	stay, ok := currentStay(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
		return
	}
	response := map[string]any{"actorType": "guest", "stay": toStayView(stay), "onlineCheckIn": nil}
	if s.lifecycle != nil {
		checkIn, err := s.lifecycle.GetOnlineCheckInByStay(r.Context(), stay.HotelID, stay.ID)
		if err == nil {
			response["onlineCheckIn"] = toOnlineCheckInView(checkIn, false)
		}
	}
	writeJSON(w, http.StatusOK, response)
}
