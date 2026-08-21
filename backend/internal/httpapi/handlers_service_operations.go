package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var serviceCodePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requestOperationRoles = []models.StaffRole{
	models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations,
	models.StaffRoleReception, models.StaffRoleHousekeeping, models.StaffRoleFB,
}

var serviceManagementRoles = []models.StaffRole{
	models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations,
}

func (s *Server) registerServiceOperationRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/guest/services", s.require(auth.ActorGuest)(http.HandlerFunc(s.listGuestServices)))
	mux.Handle("GET /api/v1/guest/requests", s.require(auth.ActorGuest)(http.HandlerFunc(s.listGuestRequests)))
	mux.Handle("POST /api/v1/guest/requests", s.require(auth.ActorGuest)(http.HandlerFunc(s.createGuestRequest)))
	mux.Handle("POST /api/v1/guest/requests/{id}/cancel", s.require(auth.ActorGuest)(http.HandlerFunc(s.cancelGuestRequest)))
	mux.Handle("GET /api/v1/staff/services", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.listStaffServices)))
	mux.Handle("POST /api/v1/staff/services", s.require(auth.ActorStaff, serviceManagementRoles...)(http.HandlerFunc(s.createService)))
	mux.Handle("PATCH /api/v1/staff/services/{id}", s.require(auth.ActorStaff, serviceManagementRoles...)(http.HandlerFunc(s.updateService)))
	mux.Handle("GET /api/v1/staff/requests", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.listStaffRequests)))
	mux.Handle("POST /api/v1/staff/requests/{id}/assign", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.assignRequest)))
	mux.Handle("POST /api/v1/staff/requests/{id}/priority", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.updateRequestPriority)))
	mux.Handle("POST /api/v1/staff/requests/{id}/transition", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.transitionRequest)))
	mux.Handle("POST /api/v1/staff/requests/{id}/notes", s.require(auth.ActorStaff, requestOperationRoles...)(http.HandlerFunc(s.addRequestNote)))
}

func (s *Server) requireServiceOperations(w http.ResponseWriter) bool {
	if s.serviceOperations == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "سرویس درخواست‌ها در دسترس نیست")
		return false
	}
	return true
}

func (s *Server) listGuestServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	stay, _ := currentStay(r)
	services, err := s.serviceOperations.ListGuestServices(r.Context(), stay.HotelID)
	if err != nil {
		s.logger.Error("list guest services", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت خدمات انجام نشد")
		return
	}
	views := make([]serviceView, 0, len(services))
	for _, service := range services {
		views = append(views, toServiceView(service))
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": views})
}

func (s *Server) listStaffServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, _ := currentStaff(r)
	services, err := s.serviceOperations.ListStaffServices(r.Context(), staff.HotelID)
	if err != nil {
		s.logger.Error("list staff services", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت خدمات انجام نشد")
		return
	}
	views := make([]serviceView, 0, len(services))
	for _, service := range services {
		views = append(views, toServiceView(service))
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": views})
}

type serviceInput struct {
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
	SortOrder        int                    `json:"sortOrder"`
	IsActive         bool                   `json:"isActive"`
}

func (input *serviceInput) normalize() {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Icon = strings.ToLower(strings.TrimSpace(input.Icon))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
}

func (input serviceInput) valid() bool {
	return serviceCodePattern.MatchString(input.Code) && len(input.Code) <= 64 && len(input.Name) >= 2 && len(input.Name) <= 120 &&
		len(input.Description) <= 500 && validServiceCategory(input.Category) && validFulfillmentRole(input.FulfillmentRole) &&
		len(input.Icon) >= 2 && len(input.Icon) <= 32 && input.EstimatedMinutes >= 1 && input.EstimatedMinutes <= 1440 &&
		input.PriceCents >= 0 && input.PriceCents <= 100_000_000_000 && len(input.Currency) == 3 && input.SortOrder >= 0 && input.SortOrder <= 10000 &&
		(input.IsPaid || input.PriceCents == 0)
}

func (input serviceInput) model(hotelID uuid.UUID) models.Service {
	return models.Service{
		HotelID: hotelID, Code: input.Code, Name: input.Name, Description: input.Description,
		Category: input.Category, Icon: input.Icon, FulfillmentRole: input.FulfillmentRole,
		EstimatedMinutes: input.EstimatedMinutes, PriceCents: input.PriceCents, Currency: input.Currency,
		IsPaid: input.IsPaid, IsQuickAction: input.IsQuickAction, SortOrder: input.SortOrder, IsActive: input.IsActive,
	}
}

func (s *Server) createService(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input serviceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.normalize()
	if !input.valid() {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات خدمت معتبر نیست")
		return
	}
	service := input.model(staff.HotelID)
	if err := s.serviceOperations.CreateService(r.Context(), &service); err != nil {
		if errors.Is(err, store.ErrServiceCodeExists) {
			writeError(w, http.StatusConflict, "service_code_exists", "کد خدمت قبلاً استفاده شده است")
			return
		}
		s.logger.Error("create service", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد خدمت انجام نشد")
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service.create", models.AuditOutcomeSuccess, map[string]any{"serviceId": service.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"service": toServiceView(service)})
}

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, _ := currentStaff(r)
	serviceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_service", "شناسه خدمت معتبر نیست")
		return
	}
	var input serviceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.normalize()
	if !input.valid() {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات خدمت معتبر نیست")
		return
	}
	service, err := s.serviceOperations.UpdateService(r.Context(), staff.HotelID, serviceID, input.model(staff.HotelID))
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service.update", models.AuditOutcomeSuccess, map[string]any{"serviceId": service.ID})
	writeJSON(w, http.StatusOK, map[string]any{"service": toServiceView(service)})
}

type createGuestRequestInput struct {
	ServiceID uuid.UUID `json:"serviceId"`
	Quantity  int       `json:"quantity"`
	Notes     string    `json:"notes"`
}

func (s *Server) createGuestRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	stay, _ := currentStay(r)
	var input createGuestRequestInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if input.ServiceID == uuid.Nil || input.Quantity < 1 || input.Quantity > 20 || len(input.Notes) > 500 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات درخواست معتبر نیست")
		return
	}
	request, err := s.serviceOperations.CreateServiceRequest(r.Context(), stay, input.ServiceID, input.Quantity, input.Notes, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "service_request.create", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID, "serviceId": request.ServiceID})
	s.publishServiceRequest("request.created", request)
	writeJSON(w, http.StatusCreated, map[string]any{"request": toServiceRequestView(request, false)})
}

func (s *Server) listGuestRequests(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	stay, _ := currentStay(r)
	requests, err := s.serviceOperations.ListGuestRequests(r.Context(), stay.HotelID, stay.ID)
	if err != nil {
		s.logger.Error("list guest requests", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت درخواست‌ها انجام نشد")
		return
	}
	views := make([]serviceRequestView, 0, len(requests))
	for _, request := range requests {
		views = append(views, toServiceRequestView(request, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": views})
}

type requestNoteInput struct {
	Note string `json:"note"`
}

func (s *Server) cancelGuestRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	stay, _ := currentStay(r)
	requestID, ok := parseRequestID(w, r)
	if !ok {
		return
	}
	var input requestNoteInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if len(input.Note) > 500 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "یادداشت بیش از حد طولانی است")
		return
	}
	request, err := s.serviceOperations.CancelGuestRequest(r.Context(), stay.HotelID, stay.ID, stay.GuestID, requestID, input.Note, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "service_request.cancel", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID})
	s.publishServiceRequest("request.updated", request)
	writeJSON(w, http.StatusOK, map[string]any{"request": toServiceRequestView(request, false)})
}

func (s *Server) listStaffRequests(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, _ := currentStaff(r)
	status := models.RequestStatus(r.URL.Query().Get("status"))
	category := models.ServiceCategory(r.URL.Query().Get("category"))
	if !validRequestStatus(status, true) || !validServiceCategoryFilter(category) {
		writeError(w, http.StatusBadRequest, "invalid_filter", "فیلتر درخواست معتبر نیست")
		return
	}
	filter := store.RequestFilter{Status: status, Category: category, Unassigned: r.URL.Query().Get("unassigned") == "true"}
	if r.URL.Query().Get("assignedToMe") == "true" {
		filter.AssignedToID = &staff.ID
		filter.Unassigned = false
	}
	if staff.Role == models.StaffRoleHousekeeping {
		filter.FulfillmentRole = models.StaffRoleHousekeeping
	}
	if staff.Role == models.StaffRoleFB {
		filter.FulfillmentRole = models.StaffRoleFB
	}
	requests, err := s.serviceOperations.ListStaffRequests(r.Context(), staff.HotelID, filter)
	if err != nil {
		s.logger.Error("list staff requests", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت صف عملیات انجام نشد")
		return
	}
	views := make([]serviceRequestView, 0, len(requests))
	for _, request := range requests {
		views = append(views, toServiceRequestView(request, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": views})
}

type assignRequestInput struct {
	StaffID *uuid.UUID `json:"staffId"`
}

func (s *Server) assignRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, request, ok := s.authorizeRequestMutation(w, r)
	if !ok {
		return
	}
	var input assignRequestInput
	if !decodeJSON(w, r, &input) {
		return
	}
	assignedToID := staff.ID
	if input.StaffID != nil {
		assignedToID = *input.StaffID
	}
	if (staff.Role == models.StaffRoleHousekeeping || staff.Role == models.StaffRoleFB) && assignedToID != staff.ID {
		writeError(w, http.StatusForbidden, "forbidden", "این نقش فقط می‌تواند درخواست را به خود اختصاص دهد")
		return
	}
	updated, err := s.serviceOperations.AssignServiceRequest(r.Context(), staff.HotelID, request.ID, staff.ID, assignedToID, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service_request.assign", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID, "assignedToId": assignedToID})
	s.publishServiceRequest("request.updated", updated)
	writeJSON(w, http.StatusOK, map[string]any{"request": toServiceRequestView(updated, true)})
}

type priorityInput struct {
	Priority int `json:"priority"`
}

func (s *Server) updateRequestPriority(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, request, ok := s.authorizeRequestMutation(w, r)
	if !ok {
		return
	}
	var input priorityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Priority < 0 || input.Priority > 3 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اولویت معتبر نیست")
		return
	}
	updated, err := s.serviceOperations.UpdateRequestPriority(r.Context(), staff.HotelID, request.ID, staff.ID, input.Priority, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service_request.priority", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID, "priority": input.Priority})
	s.publishServiceRequest("request.updated", updated)
	writeJSON(w, http.StatusOK, map[string]any{"request": toServiceRequestView(updated, true)})
}

type transitionInput struct {
	Status models.RequestStatus `json:"status"`
	Note   string               `json:"note"`
}

func (s *Server) transitionRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, request, ok := s.authorizeRequestMutation(w, r)
	if !ok {
		return
	}
	var input transitionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if !validRequestStatus(input.Status, false) || input.Status == models.RequestNew || len(input.Note) > 500 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "تغییر وضعیت معتبر نیست")
		return
	}
	updated, err := s.serviceOperations.TransitionServiceRequest(r.Context(), staff.HotelID, request.ID, staff.ID, input.Status, input.Note, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service_request.transition", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID, "status": input.Status})
	s.publishServiceRequest("request.updated", updated)
	writeJSON(w, http.StatusOK, map[string]any{"request": toServiceRequestView(updated, true)})
}

func (s *Server) addRequestNote(w http.ResponseWriter, r *http.Request) {
	if !s.requireServiceOperations(w) {
		return
	}
	staff, request, ok := s.authorizeRequestMutation(w, r)
	if !ok {
		return
	}
	var input requestNoteInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" || len(input.Note) > 500 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "یادداشت معتبر نیست")
		return
	}
	updated, err := s.serviceOperations.AddRequestNote(r.Context(), staff.HotelID, request.ID, staff.ID, input.Note, time.Now().UTC())
	if err != nil {
		writeServiceOperationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "service_request.note", models.AuditOutcomeSuccess, map[string]any{"requestId": request.ID})
	s.publishServiceRequest("request.updated", updated)
	writeJSON(w, http.StatusOK, map[string]any{"request": toServiceRequestView(updated, true)})
}

func (s *Server) authorizeRequestMutation(w http.ResponseWriter, r *http.Request) (models.StaffUser, models.ServiceRequest, bool) {
	staff, _ := currentStaff(r)
	requestID, ok := parseRequestID(w, r)
	if !ok {
		return models.StaffUser{}, models.ServiceRequest{}, false
	}
	request, err := s.serviceOperations.GetServiceRequest(r.Context(), staff.HotelID, requestID)
	if err != nil {
		writeServiceOperationError(w, err)
		return models.StaffUser{}, models.ServiceRequest{}, false
	}
	if !staffCanHandleRequest(staff.Role, request.Service.FulfillmentRole) {
		writeError(w, http.StatusForbidden, "forbidden", "این درخواست خارج از صف نقش شماست")
		return models.StaffUser{}, models.ServiceRequest{}, false
	}
	return staff, request, true
}

func staffCanHandleRequest(role, fulfillmentRole models.StaffRole) bool {
	switch role {
	case models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations, models.StaffRoleReception:
		return true
	case models.StaffRoleHousekeeping:
		return fulfillmentRole == models.StaffRoleHousekeeping
	case models.StaffRoleFB:
		return fulfillmentRole == models.StaffRoleFB
	default:
		return false
	}
}

func parseRequestID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "شناسه درخواست معتبر نیست")
		return uuid.Nil, false
	}
	return requestID, true
}

func validServiceCategory(category models.ServiceCategory) bool {
	switch category {
	case models.ServiceCategoryHousekeeping, models.ServiceCategoryFNB, models.ServiceCategoryTransport, models.ServiceCategoryWellness, models.ServiceCategoryOther:
		return true
	default:
		return false
	}
}

func validServiceCategoryFilter(category models.ServiceCategory) bool {
	return category == "" || validServiceCategory(category)
}

func validFulfillmentRole(role models.StaffRole) bool {
	return role == models.StaffRoleReception || role == models.StaffRoleHousekeeping || role == models.StaffRoleFB
}

func validRequestStatus(status models.RequestStatus, allowEmpty bool) bool {
	if allowEmpty && status == "" {
		return true
	}
	switch status {
	case models.RequestNew, models.RequestInProgress, models.RequestCompleted, models.RequestCancelled:
		return true
	default:
		return false
	}
}

func writeServiceOperationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "not_found", "مورد درخواستی پیدا نشد")
	case errors.Is(err, store.ErrServiceCodeExists):
		writeError(w, http.StatusConflict, "service_code_exists", "کد خدمت قبلاً استفاده شده است")
	case errors.Is(err, store.ErrAssignmentRoleMismatch):
		writeError(w, http.StatusConflict, "assignment_role_mismatch", "نقش حساب انتخاب‌شده با نوع خدمت سازگار نیست")
	case errors.Is(err, store.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_transition", "تغییر وضعیت در شرایط فعلی مجاز نیست")
	default:
		writeError(w, http.StatusInternalServerError, "internal_server_error", "عملیات درخواست انجام نشد")
	}
}

func (s *Server) publishServiceRequest(eventType string, request models.ServiceRequest) {
	if s.realtime == nil {
		return
	}
	s.realtime.Publish(realtime.Event{
		Type: eventType, HotelID: request.HotelID, StayID: request.StayID,
		Category: request.Service.Category, FulfillmentRole: request.Service.FulfillmentRole,
		Payload: map[string]any{"request": toServiceRequestView(request, true)}, EmittedAt: time.Now().UTC(),
	})
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(value)
	return parsed
}
