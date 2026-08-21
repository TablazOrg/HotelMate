package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"gorm.io/gorm"
)

func (s *Server) publicHotel(w http.ResponseWriter, r *http.Request) {
	slug := normalizeSlug(r.PathValue("slug"))
	if !slugPattern.MatchString(slug) || s.store == nil {
		writeError(w, http.StatusNotFound, "hotel_not_found", "هتل پیدا نشد")
		return
	}
	hotel, err := s.store.FindHotelBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hotel_not_found", "هتل پیدا نشد")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hotel": toHotelView(hotel)})
}

type onboardingRequest struct {
	Hotel struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		LogoURL      string `json:"logoUrl"`
		PrimaryColor string `json:"primaryColor"`
		Timezone     string `json:"timezone"`
	} `json:"hotel"`
	PrimaryAdmin struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	} `json:"primaryAdmin"`
}

func (s *Server) onboardHotel(w http.ResponseWriter, r *http.Request) {
	if !s.validOnboardingToken(r.Header.Get("X-Onboarding-Token")) {
		s.audit(r, nil, nil, "system", "hotel.onboard", models.AuditOutcomeFailure, map[string]any{"reason": "invalid_token"})
		writeError(w, http.StatusUnauthorized, "unauthorized", "توکن راه‌اندازی معتبر نیست")
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس در دسترس نیست")
		return
	}
	var input onboardingRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Hotel.Name = strings.TrimSpace(input.Hotel.Name)
	input.Hotel.Slug = normalizeSlug(input.Hotel.Slug)
	input.Hotel.PrimaryColor = strings.TrimSpace(input.Hotel.PrimaryColor)
	input.Hotel.Timezone = strings.TrimSpace(input.Hotel.Timezone)
	input.PrimaryAdmin.FirstName = strings.TrimSpace(input.PrimaryAdmin.FirstName)
	input.PrimaryAdmin.LastName = strings.TrimSpace(input.PrimaryAdmin.LastName)
	input.PrimaryAdmin.Email = normalizeEmail(input.PrimaryAdmin.Email)
	if input.Hotel.PrimaryColor == "" {
		input.Hotel.PrimaryColor = "#0f766e"
	}
	if input.Hotel.Timezone == "" {
		input.Hotel.Timezone = "Asia/Tehran"
	}
	if len(input.Hotel.Name) < 2 || len(input.Hotel.Name) > 120 || !slugPattern.MatchString(input.Hotel.Slug) ||
		!colorPattern.MatchString(input.Hotel.PrimaryColor) || len(input.Hotel.Timezone) > 64 ||
		len(input.PrimaryAdmin.FirstName) < 2 || len(input.PrimaryAdmin.LastName) < 2 || !validEmail(input.PrimaryAdmin.Email) ||
		!validOptionalHTTPURL(input.Hotel.LogoURL) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات هتل یا مدیر معتبر نیست")
		return
	}
	passwordHash, err := auth.HashPassword(input.PrimaryAdmin.Password)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "رمز عبور باید حداقل ۱۲ کاراکتر باشد")
		return
	}
	onboarding := store.HotelOnboarding{
		Hotel:        models.Hotel{Name: input.Hotel.Name, Slug: input.Hotel.Slug, LogoURL: strings.TrimSpace(input.Hotel.LogoURL), PrimaryColor: input.Hotel.PrimaryColor, Timezone: input.Hotel.Timezone},
		PrimaryAdmin: models.StaffUser{FirstName: input.PrimaryAdmin.FirstName, LastName: input.PrimaryAdmin.LastName, Email: input.PrimaryAdmin.Email, PasswordHash: passwordHash, Role: models.StaffRolePrimaryAdmin, IsActive: true},
	}
	if err := s.store.CreateHotelWithPrimaryAdmin(r.Context(), &onboarding); err != nil {
		s.audit(r, nil, nil, "system", "hotel.onboard", models.AuditOutcomeFailure, map[string]any{"slug": input.Hotel.Slug})
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "hotel_exists", "شناسه هتل یا ایمیل مدیر قبلاً استفاده شده است")
			return
		}
		s.logger.Error("onboard hotel", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد هتل انجام نشد")
		return
	}
	s.audit(r, &onboarding.Hotel.ID, &onboarding.PrimaryAdmin.ID, "staff", "hotel.onboard", models.AuditOutcomeSuccess, map[string]any{"slug": onboarding.Hotel.Slug})
	writeJSON(w, http.StatusCreated, map[string]any{"hotel": toHotelView(onboarding.Hotel), "primaryAdmin": toStaffView(onboarding.PrimaryAdmin)})
}

func validOptionalHTTPURL(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
