package httpapi

import (
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

var contentManagementRoles = []models.StaffRole{models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations}

func (s *Server) registerContentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/public/hotels/{slug}/content", s.publicHotelContent)
	mux.Handle("GET /api/v1/staff/content", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.staffHotelContent)))
	mux.Handle("POST /api/v1/staff/facilities", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.createFacility)))
	mux.Handle("PATCH /api/v1/staff/facilities/{id}", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.updateFacility)))
	mux.Handle("POST /api/v1/staff/promotions", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.createPromotion)))
	mux.Handle("PATCH /api/v1/staff/promotions/{id}", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.updatePromotion)))
	mux.Handle("POST /api/v1/staff/restaurants", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.createRestaurant)))
	mux.Handle("PATCH /api/v1/staff/restaurants/{id}", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.updateRestaurant)))
	mux.Handle("POST /api/v1/staff/restaurants/{id}/menu-items", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.createMenuItem)))
	mux.Handle("PATCH /api/v1/staff/menu-items/{id}", s.require(auth.ActorStaff, contentManagementRoles...)(http.HandlerFunc(s.updateMenuItem)))
}

func (s *Server) requireContent(w http.ResponseWriter) bool {
	if s.content == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "محتوای هتل در دسترس نیست")
		return false
	}
	return true
}

func contentPayload(content store.HotelContent) map[string]any {
	if content.Facilities == nil {
		content.Facilities = []models.Facility{}
	}
	if content.Promotions == nil {
		content.Promotions = []models.Promotion{}
	}
	if content.Restaurants == nil {
		content.Restaurants = []models.Restaurant{}
	}
	for index := range content.Restaurants {
		if content.Restaurants[index].MenuItems == nil {
			content.Restaurants[index].MenuItems = []models.MenuItem{}
		}
	}
	return map[string]any{"hotel": toHotelView(content.Hotel), "facilities": content.Facilities, "promotions": content.Promotions, "restaurants": content.Restaurants}
}

func (s *Server) publicHotelContent(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("slug")))
	if !slugPattern.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "invalid_hotel", "شناسه هتل معتبر نیست")
		return
	}
	content, err := s.content.GetPublicContent(r.Context(), slug, time.Now().UTC())
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contentPayload(content))
}

func (s *Server) staffHotelContent(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	content, err := s.content.GetStaffContent(r.Context(), staff.HotelID)
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contentPayload(content))
}

type facilityInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Hours       string `json:"hours"`
	SortOrder   int    `json:"sortOrder"`
	IsActive    bool   `json:"isActive"`
}

func (input *facilityInput) model(hotelID uuid.UUID) (models.Facility, bool) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Icon = strings.ToLower(strings.TrimSpace(input.Icon))
	input.Hours = strings.TrimSpace(input.Hours)
	valid := len(input.Name) >= 2 && len(input.Name) <= 120 && len(input.Description) <= 500 && len(input.Icon) >= 2 && len(input.Icon) <= 32 && len(input.Hours) <= 120 && input.SortOrder >= 0 && input.SortOrder <= 10000
	return models.Facility{HotelID: hotelID, Name: input.Name, Description: input.Description, Icon: input.Icon, Hours: input.Hours, SortOrder: input.SortOrder, IsActive: input.IsActive}, valid
}

func (s *Server) createFacility(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input facilityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	facility, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات امکان هتل معتبر نیست")
		return
	}
	if err := s.content.CreateFacility(r.Context(), &facility); err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "facility.create", models.AuditOutcomeSuccess, map[string]any{"facilityId": facility.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"facility": facility})
}

func (s *Server) updateFacility(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseContentID(w, r)
	if !ok {
		return
	}
	var input facilityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	facility, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات امکان هتل معتبر نیست")
		return
	}
	updated, err := s.content.UpdateFacility(r.Context(), staff.HotelID, id, facility)
	if err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "facility.update", models.AuditOutcomeSuccess, map[string]any{"facilityId": id})
	writeJSON(w, http.StatusOK, map[string]any{"facility": updated})
}

type promotionInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	DiscountPct float64 `json:"discountPct"`
	BadgeText   string  `json:"badgeText"`
	StartsAt    string  `json:"startsAt"`
	EndsAt      string  `json:"endsAt"`
	IsActive    bool    `json:"isActive"`
}

func (input *promotionInput) model(hotelID uuid.UUID) (models.Promotion, bool) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.BadgeText = strings.TrimSpace(input.BadgeText)
	starts, startErr := time.Parse(time.RFC3339, input.StartsAt)
	ends, endErr := time.Parse(time.RFC3339, input.EndsAt)
	valid := len(input.Title) >= 2 && len(input.Title) <= 140 && len(input.Description) <= 700 && len(input.BadgeText) <= 32 && input.DiscountPct >= 0 && input.DiscountPct <= 100 && startErr == nil && endErr == nil && ends.After(starts)
	return models.Promotion{HotelID: hotelID, Title: input.Title, Description: input.Description, DiscountPct: input.DiscountPct, BadgeText: input.BadgeText, StartsAt: starts.UTC(), EndsAt: ends.UTC(), IsActive: input.IsActive}, valid
}

func (s *Server) createPromotion(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input promotionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	promotion, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات پیشنهاد معتبر نیست")
		return
	}
	if err := s.content.CreatePromotion(r.Context(), &promotion); err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "promotion.create", models.AuditOutcomeSuccess, map[string]any{"promotionId": promotion.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"promotion": promotion})
}

func (s *Server) updatePromotion(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseContentID(w, r)
	if !ok {
		return
	}
	var input promotionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	promotion, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات پیشنهاد معتبر نیست")
		return
	}
	updated, err := s.content.UpdatePromotion(r.Context(), staff.HotelID, id, promotion)
	if err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "promotion.update", models.AuditOutcomeSuccess, map[string]any{"promotionId": id})
	writeJSON(w, http.StatusOK, map[string]any{"promotion": updated})
}

type restaurantInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Hours       string `json:"hours"`
	SortOrder   int    `json:"sortOrder"`
	IsActive    bool   `json:"isActive"`
}

func (input *restaurantInput) model(hotelID uuid.UUID) (models.Restaurant, bool) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Hours = strings.TrimSpace(input.Hours)
	valid := len(input.Name) >= 2 && len(input.Name) <= 120 && len(input.Description) <= 500 && len(input.Hours) <= 120 && input.SortOrder >= 0 && input.SortOrder <= 10000
	return models.Restaurant{HotelID: hotelID, Name: input.Name, Description: input.Description, Hours: input.Hours, SortOrder: input.SortOrder, IsActive: input.IsActive}, valid
}

func (s *Server) createRestaurant(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input restaurantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	restaurant, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات رستوران معتبر نیست")
		return
	}
	if err := s.content.CreateRestaurant(r.Context(), &restaurant); err != nil {
		writeContentError(w, err)
		return
	}
	restaurant.MenuItems = []models.MenuItem{}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "restaurant.create", models.AuditOutcomeSuccess, map[string]any{"restaurantId": restaurant.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"restaurant": restaurant})
}

func (s *Server) updateRestaurant(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseContentID(w, r)
	if !ok {
		return
	}
	var input restaurantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	restaurant, valid := input.model(staff.HotelID)
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات رستوران معتبر نیست")
		return
	}
	updated, err := s.content.UpdateRestaurant(r.Context(), staff.HotelID, id, restaurant)
	if err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "restaurant.update", models.AuditOutcomeSuccess, map[string]any{"restaurantId": id})
	writeJSON(w, http.StatusOK, map[string]any{"restaurant": updated})
}

type menuItemInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"priceCents"`
	Currency    string `json:"currency"`
	SortOrder   int    `json:"sortOrder"`
	IsAvailable bool   `json:"isAvailable"`
}

func (input *menuItemInput) model() (models.MenuItem, bool) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	valid := len(input.Name) >= 2 && len(input.Name) <= 140 && len(input.Description) <= 500 && input.PriceCents >= 0 && input.PriceCents <= 100_000_000_000 && len(input.Currency) == 3 && input.SortOrder >= 0 && input.SortOrder <= 10000
	return models.MenuItem{Name: input.Name, Description: input.Description, PriceCents: input.PriceCents, Currency: input.Currency, SortOrder: input.SortOrder, IsAvailable: input.IsAvailable}, valid
}

func (s *Server) createMenuItem(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	restaurantID, ok := parseContentID(w, r)
	if !ok {
		return
	}
	var input menuItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, valid := input.model()
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات آیتم منو معتبر نیست")
		return
	}
	if err := s.content.CreateMenuItem(r.Context(), staff.HotelID, restaurantID, &item); err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "menu_item.create", models.AuditOutcomeSuccess, map[string]any{"menuItemId": item.ID, "restaurantId": restaurantID})
	writeJSON(w, http.StatusCreated, map[string]any{"menuItem": item})
}

func (s *Server) updateMenuItem(w http.ResponseWriter, r *http.Request) {
	if !s.requireContent(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseContentID(w, r)
	if !ok {
		return
	}
	var input menuItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, valid := input.model()
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات آیتم منو معتبر نیست")
		return
	}
	updated, err := s.content.UpdateMenuItem(r.Context(), staff.HotelID, id, item)
	if err != nil {
		writeContentError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "menu_item.update", models.AuditOutcomeSuccess, map[string]any{"menuItemId": id})
	writeJSON(w, http.StatusOK, map[string]any{"menuItem": updated})
}

func parseContentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "شناسه معتبر نیست")
		return uuid.Nil, false
	}
	return id, true
}

func writeContentError(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "مورد درخواستی پیدا نشد")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_server_error", "عملیات محتوای هتل انجام نشد")
}
