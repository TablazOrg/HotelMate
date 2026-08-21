package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrServiceCodeExists      = errors.New("service code already exists")
	ErrAssignmentRoleMismatch = errors.New("staff role cannot fulfill this service")
)

type RequestFilter struct {
	Status          models.RequestStatus
	Category        models.ServiceCategory
	FulfillmentRole models.StaffRole
	AssignedToID    *uuid.UUID
	Unassigned      bool
}

type ServiceOperationsStore interface {
	ListGuestServices(context.Context, uuid.UUID) ([]models.Service, error)
	ListStaffServices(context.Context, uuid.UUID) ([]models.Service, error)
	CreateService(context.Context, *models.Service) error
	UpdateService(context.Context, uuid.UUID, uuid.UUID, models.Service) (models.Service, error)
	CreateServiceRequest(context.Context, models.Stay, uuid.UUID, int, string, time.Time) (models.ServiceRequest, error)
	ListGuestRequests(context.Context, uuid.UUID, uuid.UUID) ([]models.ServiceRequest, error)
	ListStaffRequests(context.Context, uuid.UUID, RequestFilter) ([]models.ServiceRequest, error)
	GetServiceRequest(context.Context, uuid.UUID, uuid.UUID) (models.ServiceRequest, error)
	AssignServiceRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (models.ServiceRequest, error)
	UpdateRequestPriority(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, time.Time) (models.ServiceRequest, error)
	TransitionServiceRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.RequestStatus, string, time.Time) (models.ServiceRequest, error)
	AddRequestNote(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) (models.ServiceRequest, error)
	CancelGuestRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) (models.ServiceRequest, error)
}

func (s *GORMStore) ListGuestServices(ctx context.Context, hotelID uuid.UUID) ([]models.Service, error) {
	var services []models.Service
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND is_active = ?", hotelID, true).
		Order("is_quick_action DESC, sort_order ASC, name ASC").Find(&services).Error
	return services, err
}

func (s *GORMStore) ListStaffServices(ctx context.Context, hotelID uuid.UUID) ([]models.Service, error) {
	var services []models.Service
	err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).
		Order("is_active DESC, sort_order ASC, name ASC").Find(&services).Error
	return services, err
}

func (s *GORMStore) CreateService(ctx context.Context, service *models.Service) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Service{}).
		Where("hotel_id = ? AND code = ?", service.HotelID, service.Code).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrServiceCodeExists
	}
	if err := s.db.WithContext(ctx).Create(service).Error; errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrServiceCodeExists
	} else {
		return err
	}
}

func (s *GORMStore) UpdateService(ctx context.Context, hotelID, serviceID uuid.UUID, input models.Service) (models.Service, error) {
	var service models.Service
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, serviceID).First(&service).Error; err != nil {
			return err
		}
		if input.Code != service.Code {
			var count int64
			if err := tx.Model(&models.Service{}).Where("hotel_id = ? AND code = ? AND id <> ?", hotelID, input.Code, serviceID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrServiceCodeExists
			}
		}
		updates := map[string]any{
			"code": input.Code, "name": input.Name, "description": input.Description,
			"category": input.Category, "icon": input.Icon, "fulfillment_role": input.FulfillmentRole,
			"estimated_minutes": input.EstimatedMinutes, "price_cents": input.PriceCents,
			"currency": input.Currency, "is_paid": input.IsPaid, "is_quick_action": input.IsQuickAction,
			"sort_order": input.SortOrder, "is_active": input.IsActive,
		}
		return tx.Model(&service).Updates(updates).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return models.Service{}, ErrServiceCodeExists
		}
		return models.Service{}, err
	}
	return s.findService(ctx, hotelID, serviceID)
}

func (s *GORMStore) CreateServiceRequest(ctx context.Context, stay models.Stay, serviceID uuid.UUID, quantity int, notes string, at time.Time) (models.ServiceRequest, error) {
	var request models.ServiceRequest
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedStay models.Stay
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ? AND guest_id = ?", stay.HotelID, stay.ID, stay.GuestID).First(&lockedStay).Error; err != nil {
			return err
		}
		if lockedStay.Status != models.StayActive {
			return ErrInvalidTransition
		}
		var service models.Service
		if err := tx.Where("hotel_id = ? AND id = ? AND is_active = ?", stay.HotelID, serviceID, true).First(&service).Error; err != nil {
			return err
		}
		request = models.ServiceRequest{
			HotelID: stay.HotelID, StayID: stay.ID, ServiceID: service.ID, Status: models.RequestNew,
			Quantity: quantity, Notes: notes, TotalPriceCents: service.PriceCents * int64(quantity), Priority: 0,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		event := models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   stay.HotelID, RequestID: request.ID, EventType: models.RequestEventCreated,
			ActorType: "guest", ActorID: stay.GuestID, Note: strings.TrimSpace(notes),
		}
		return tx.Create(&event).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, stay.HotelID, request.ID)
}

func (s *GORMStore) ListGuestRequests(ctx context.Context, hotelID, stayID uuid.UUID) ([]models.ServiceRequest, error) {
	var requests []models.ServiceRequest
	err := s.requestPreloads(s.db.WithContext(ctx)).
		Where("service_requests.hotel_id = ? AND service_requests.stay_id = ?", hotelID, stayID).
		Order("service_requests.created_at DESC").Find(&requests).Error
	return requests, err
}

func (s *GORMStore) ListStaffRequests(ctx context.Context, hotelID uuid.UUID, filter RequestFilter) ([]models.ServiceRequest, error) {
	query := s.requestPreloads(s.db.WithContext(ctx)).Where("service_requests.hotel_id = ?", hotelID)
	if filter.Status != "" {
		query = query.Where("service_requests.status = ?", filter.Status)
	}
	if filter.Category != "" || filter.FulfillmentRole != "" {
		query = query.Joins("JOIN services request_filter_services ON request_filter_services.id = service_requests.service_id")
	}
	if filter.Category != "" {
		query = query.Where("request_filter_services.category = ?", filter.Category)
	}
	if filter.FulfillmentRole != "" {
		query = query.Where("request_filter_services.fulfillment_role = ?", filter.FulfillmentRole)
	}
	if filter.AssignedToID != nil {
		query = query.Where("service_requests.assigned_to_id = ?", *filter.AssignedToID)
	}
	if filter.Unassigned {
		query = query.Where("service_requests.assigned_to_id IS NULL")
	}
	var requests []models.ServiceRequest
	err := query.Order("service_requests.priority DESC, service_requests.created_at ASC").Find(&requests).Error
	return requests, err
}

func (s *GORMStore) GetServiceRequest(ctx context.Context, hotelID, requestID uuid.UUID) (models.ServiceRequest, error) {
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) AssignServiceRequest(ctx context.Context, hotelID, requestID, actorID, assignedToID uuid.UUID, at time.Time) (models.ServiceRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ServiceRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, requestID).First(&request).Error; err != nil {
			return err
		}
		if request.Status == models.RequestCompleted || request.Status == models.RequestCancelled {
			return ErrInvalidTransition
		}
		var staff models.StaffUser
		if err := tx.Where("hotel_id = ? AND id = ? AND is_active = ?", hotelID, assignedToID, true).First(&staff).Error; err != nil {
			return err
		}
		var service models.Service
		if err := tx.Where("hotel_id = ? AND id = ?", hotelID, request.ServiceID).First(&service).Error; err != nil {
			return err
		}
		if staff.Role != service.FulfillmentRole && staff.Role != models.StaffRolePrimaryAdmin && staff.Role != models.StaffRoleSecondaryAdmin && staff.Role != models.StaffRoleOperations {
			return ErrAssignmentRoleMismatch
		}
		if err := tx.Model(&request).Update("assigned_to_id", assignedToID).Error; err != nil {
			return err
		}
		return tx.Create(&models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   hotelID, RequestID: requestID, EventType: models.RequestEventAssigned,
			ActorType: "staff", ActorID: actorID, Note: staff.FirstName + " " + staff.LastName,
		}).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) UpdateRequestPriority(ctx context.Context, hotelID, requestID, actorID uuid.UUID, priority int, at time.Time) (models.ServiceRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ServiceRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, requestID).First(&request).Error; err != nil {
			return err
		}
		if request.Status == models.RequestCompleted || request.Status == models.RequestCancelled {
			return ErrInvalidTransition
		}
		if err := tx.Model(&request).Update("priority", priority).Error; err != nil {
			return err
		}
		return tx.Create(&models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   hotelID, RequestID: requestID, EventType: models.RequestEventPriority,
			ActorType: "staff", ActorID: actorID, Note: strings.TrimSpace(priorityLabel(priority)),
		}).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) TransitionServiceRequest(ctx context.Context, hotelID, requestID, actorID uuid.UUID, status models.RequestStatus, note string, at time.Time) (models.ServiceRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ServiceRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, requestID).First(&request).Error; err != nil {
			return err
		}
		if !validRequestTransition(request.Status, status) {
			return ErrInvalidTransition
		}
		from := request.Status
		updates := map[string]any{"status": status}
		if status == models.RequestInProgress {
			updates["started_at"] = at
			if request.AssignedToID == nil {
				updates["assigned_to_id"] = actorID
			}
		}
		if status == models.RequestCompleted {
			updates["completed_at"] = at
		}
		if status == models.RequestCancelled {
			updates["cancelled_at"] = at
		}
		if err := tx.Model(&request).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   hotelID, RequestID: requestID, EventType: models.RequestEventStatus,
			FromStatus: &from, ToStatus: &status, ActorType: "staff", ActorID: actorID,
			Note: strings.TrimSpace(note),
		}).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) AddRequestNote(ctx context.Context, hotelID, requestID, actorID uuid.UUID, note string, at time.Time) (models.ServiceRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ServiceRequest
		if err := tx.Where("hotel_id = ? AND id = ?", hotelID, requestID).First(&request).Error; err != nil {
			return err
		}
		if err := tx.Model(&request).Update("updated_at", at).Error; err != nil {
			return err
		}
		return tx.Create(&models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   hotelID, RequestID: requestID, EventType: models.RequestEventNote,
			ActorType: "staff", ActorID: actorID, Note: strings.TrimSpace(note),
		}).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) CancelGuestRequest(ctx context.Context, hotelID, stayID, guestID, requestID uuid.UUID, note string, at time.Time) (models.ServiceRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ServiceRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hotel_id = ? AND stay_id = ? AND id = ?", hotelID, stayID, requestID).First(&request).Error; err != nil {
			return err
		}
		if request.Status != models.RequestNew {
			return ErrInvalidTransition
		}
		from, status := request.Status, models.RequestCancelled
		if err := tx.Model(&request).Updates(map[string]any{"status": status, "cancelled_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&models.ServiceRequestEvent{
			BaseModel: models.BaseModel{CreatedAt: at},
			HotelID:   hotelID, RequestID: requestID, EventType: models.RequestEventStatus,
			FromStatus: &from, ToStatus: &status, ActorType: "guest", ActorID: guestID,
			Note: strings.TrimSpace(note),
		}).Error
	})
	if err != nil {
		return models.ServiceRequest{}, err
	}
	return s.loadServiceRequest(ctx, hotelID, requestID)
}

func (s *GORMStore) findService(ctx context.Context, hotelID, serviceID uuid.UUID) (models.Service, error) {
	var service models.Service
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, serviceID).First(&service).Error
	return service, err
}

func (s *GORMStore) loadServiceRequest(ctx context.Context, hotelID, requestID uuid.UUID) (models.ServiceRequest, error) {
	var request models.ServiceRequest
	err := s.requestPreloads(s.db.WithContext(ctx)).Where("service_requests.hotel_id = ? AND service_requests.id = ?", hotelID, requestID).First(&request).Error
	return request, err
}

func (s *GORMStore) requestPreloads(query *gorm.DB) *gorm.DB {
	return query.Preload("Service").Preload("Stay.Guest").Preload("Stay.Room").Preload("AssignedTo").
		Preload("Events", func(events *gorm.DB) *gorm.DB { return events.Order("created_at ASC") })
}

func validRequestTransition(from, to models.RequestStatus) bool {
	switch from {
	case models.RequestNew:
		return to == models.RequestInProgress || to == models.RequestCancelled
	case models.RequestInProgress:
		return to == models.RequestCompleted || to == models.RequestCancelled
	default:
		return false
	}
}

func priorityLabel(priority int) string {
	switch priority {
	case 1:
		return "normal"
	case 2:
		return "high"
	case 3:
		return "urgent"
	default:
		return "low"
	}
}

var _ ServiceOperationsStore = (*GORMStore)(nil)
