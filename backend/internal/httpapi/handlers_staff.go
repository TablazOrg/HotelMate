package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"gorm.io/gorm"
)

func (s *Server) listStaff(w http.ResponseWriter, r *http.Request) {
	claims := currentClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
		return
	}
	hotelID, _ := claims.TenantID()
	staff, err := s.store.ListStaff(r.Context(), hotelID)
	if err != nil {
		s.logger.Error("list staff", "hotelId", hotelID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت حساب‌ها انجام نشد")
		return
	}
	views := make([]staffView, 0, len(staff))
	for _, user := range staff {
		views = append(views, toStaffView(user))
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": views})
}

type createStaffRequest struct {
	FirstName string           `json:"firstName"`
	LastName  string           `json:"lastName"`
	Email     string           `json:"email"`
	Password  string           `json:"password"`
	Role      models.StaffRole `json:"role"`
}

func (s *Server) createStaff(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentStaff(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
		return
	}
	var input createStaffRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = normalizeEmail(input.Email)
	if len(input.FirstName) < 2 || len(input.LastName) < 2 || !validEmail(input.Email) || !validRole(input.Role) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات حساب معتبر نیست")
		return
	}
	if actor.Role == models.StaffRoleSecondaryAdmin && (input.Role == models.StaffRolePrimaryAdmin || input.Role == models.StaffRoleSecondaryAdmin) {
		writeError(w, http.StatusForbidden, "forbidden", "فقط مدیر اصلی می‌تواند حساب مدیریتی ایجاد کند")
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "رمز عبور باید حداقل ۱۲ کاراکتر باشد")
		return
	}
	staff := models.StaffUser{
		HotelID: actor.HotelID, FirstName: input.FirstName, LastName: input.LastName,
		Email: input.Email, PasswordHash: passwordHash, Role: input.Role, IsActive: true,
	}
	if err := s.store.CreateStaff(r.Context(), &staff); err != nil {
		s.audit(r, &actor.HotelID, &actor.ID, "staff", "staff.create", models.AuditOutcomeFailure, map[string]any{"role": input.Role})
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "staff_exists", "این ایمیل قبلاً برای هتل ثبت شده است")
			return
		}
		s.logger.Error("create staff", "hotelId", actor.HotelID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد حساب انجام نشد")
		return
	}
	s.audit(r, &actor.HotelID, &actor.ID, "staff", "staff.create", models.AuditOutcomeSuccess, map[string]any{"createdStaffId": staff.ID, "role": staff.Role})
	writeJSON(w, http.StatusCreated, map[string]any{"staff": toStaffView(staff)})
}

type updateHotelRequest struct {
	Name         string `json:"name"`
	LogoURL      string `json:"logoUrl"`
	PrimaryColor string `json:"primaryColor"`
	Timezone     string `json:"timezone"`
}

func (s *Server) updateHotelBranding(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentStaff(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
		return
	}
	var input updateHotelRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.LogoURL = strings.TrimSpace(input.LogoURL)
	input.PrimaryColor = strings.TrimSpace(input.PrimaryColor)
	input.Timezone = strings.TrimSpace(input.Timezone)
	if len(input.Name) < 2 || len(input.Name) > 120 || !validOptionalHTTPURL(input.LogoURL) ||
		!colorPattern.MatchString(input.PrimaryColor) || input.Timezone == "" || len(input.Timezone) > 64 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "تنظیمات برند معتبر نیست")
		return
	}
	hotel, err := s.store.UpdateHotelBranding(r.Context(), actor.HotelID, input.Name, input.LogoURL, input.PrimaryColor, input.Timezone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "hotel_not_found", "هتل پیدا نشد")
			return
		}
		s.logger.Error("update hotel branding", "hotelId", actor.HotelID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ذخیره تنظیمات انجام نشد")
		return
	}
	s.audit(r, &actor.HotelID, &actor.ID, "staff", "hotel.branding.update", models.AuditOutcomeSuccess, map[string]any{"primaryColor": hotel.PrimaryColor})
	writeJSON(w, http.StatusOK, map[string]any{"hotel": toHotelView(hotel)})
}
