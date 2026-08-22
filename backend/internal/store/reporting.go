package store

import (
	"context"
	"sort"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

type ReportSummary struct {
	RequestsCreated           int64 `json:"requestsCreated"`
	OpenRequests              int64 `json:"openRequests"`
	CompletedRequests         int64 `json:"completedRequests"`
	AverageFulfillmentMinutes int64 `json:"averageFulfillmentMinutes"`
	PaidOrders                int64 `json:"paidOrders"`
	OrderedRevenueCents       int64 `json:"orderedRevenueCents"`
	RecognizedRevenueCents    int64 `json:"recognizedRevenueCents"`
	ActiveRooms               int64 `json:"activeRooms"`
	TotalRooms                int64 `json:"totalRooms"`
	HandedOffConversations    int64 `json:"handedOffConversations"`
	PendingKnowledge          int64 `json:"pendingKnowledge"`
	FailedSecurityEvents      int64 `json:"failedSecurityEvents"`
}

type DailyReportPoint struct {
	Date         string `json:"date"`
	Requests     int64  `json:"requests"`
	Completed    int64  `json:"completed"`
	RevenueCents int64  `json:"revenueCents"`
}

type ServiceReportRow struct {
	ServiceID              uuid.UUID `json:"serviceId"`
	ServiceName            string    `json:"serviceName"`
	Orders                 int64     `json:"orders"`
	Quantity               int64     `json:"quantity"`
	OrderedRevenueCents    int64     `json:"orderedRevenueCents"`
	RecognizedRevenueCents int64     `json:"recognizedRevenueCents"`
}

type OperationalReport struct {
	From      time.Time          `json:"from"`
	To        time.Time          `json:"to"`
	Timezone  string             `json:"timezone"`
	Currency  string             `json:"currency"`
	Summary   ReportSummary      `json:"summary"`
	Daily     []DailyReportPoint `json:"daily"`
	ByService []ServiceReportRow `json:"byService"`
}

type AuditFilter struct {
	From    time.Time
	To      time.Time
	Action  string
	Outcome models.AuditOutcome
	Limit   int
	Offset  int
}

type AuditPage struct {
	Items  []models.AuditLog `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type ReportingStore interface {
	BuildOperationalReport(context.Context, uuid.UUID, time.Time, time.Time, *time.Location) (OperationalReport, error)
	ListAuditLogs(context.Context, uuid.UUID, AuditFilter) (AuditPage, error)
}

func (s *GORMStore) BuildOperationalReport(ctx context.Context, hotelID uuid.UUID, from, to time.Time, location *time.Location) (OperationalReport, error) {
	if location == nil {
		location = time.UTC
	}
	report := OperationalReport{From: from, To: to, Timezone: location.String(), Currency: "IRR", Daily: []DailyReportPoint{}, ByService: []ServiceReportRow{}}
	query := s.db.WithContext(ctx).Where("service_requests.hotel_id = ? AND ((service_requests.created_at >= ? AND service_requests.created_at < ?) OR (service_requests.completed_at >= ? AND service_requests.completed_at < ?))", hotelID, from, to, from, to)
	var requests []models.ServiceRequest
	if err := query.Preload("Service").Find(&requests).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ServiceRequest{}).Where("hotel_id = ? AND status IN ?", hotelID, []models.RequestStatus{models.RequestNew, models.RequestInProgress}).Count(&report.Summary.OpenRequests).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.Stay{}).Where("hotel_id = ? AND status = ?", hotelID, models.StayActive).Count(&report.Summary.ActiveRooms).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.Room{}).Where("hotel_id = ?", hotelID).Count(&report.Summary.TotalRooms).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.Conversation{}).Where("hotel_id = ? AND status = ?", hotelID, models.ConversationHandedOff).Count(&report.Summary.HandedOffConversations).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.KnowledgeItem{}).Where("hotel_id = ? AND status = ?", hotelID, models.KnowledgePending).Count(&report.Summary.PendingKnowledge).Error; err != nil {
		return report, err
	}
	if err := s.db.WithContext(ctx).Model(&models.AuditLog{}).Where("hotel_id = ? AND outcome = ? AND created_at >= ? AND created_at < ?", hotelID, models.AuditOutcomeFailure, from, to).Count(&report.Summary.FailedSecurityEvents).Error; err != nil {
		return report, err
	}

	daily := make(map[string]*DailyReportPoint)
	for cursor := from.In(location); cursor.Before(to.In(location)); cursor = cursor.AddDate(0, 0, 1) {
		date := cursor.Format("2006-01-02")
		daily[date] = &DailyReportPoint{Date: date}
	}
	services := make(map[uuid.UUID]*ServiceReportRow)
	var fulfillmentTotal, fulfillmentCount int64
	for _, request := range requests {
		row := services[request.ServiceID]
		if row == nil {
			row = &ServiceReportRow{ServiceID: request.ServiceID, ServiceName: request.Service.Name}
			services[request.ServiceID] = row
		}
		createdInRange := !request.CreatedAt.Before(from) && request.CreatedAt.Before(to)
		completedInRange := request.CompletedAt != nil && !request.CompletedAt.Before(from) && request.CompletedAt.Before(to)
		if createdInRange {
			report.Summary.RequestsCreated++
			if point := daily[request.CreatedAt.In(location).Format("2006-01-02")]; point != nil {
				point.Requests++
			}
			if request.Status != models.RequestCancelled {
				row.Orders++
				row.Quantity += int64(request.Quantity)
				row.OrderedRevenueCents += request.TotalPriceCents
				if request.TotalPriceCents > 0 {
					report.Summary.PaidOrders++
					report.Summary.OrderedRevenueCents += request.TotalPriceCents
				}
			}
		}
		if completedInRange {
			report.Summary.CompletedRequests++
			report.Summary.RecognizedRevenueCents += request.TotalPriceCents
			row.RecognizedRevenueCents += request.TotalPriceCents
			if point := daily[request.CompletedAt.In(location).Format("2006-01-02")]; point != nil {
				point.Completed++
				point.RevenueCents += request.TotalPriceCents
			}
			if request.StartedAt != nil && request.CompletedAt.After(*request.StartedAt) {
				fulfillmentTotal += int64(request.CompletedAt.Sub(*request.StartedAt).Minutes())
				fulfillmentCount++
			}
		}
	}
	if fulfillmentCount > 0 {
		report.Summary.AverageFulfillmentMinutes = fulfillmentTotal / fulfillmentCount
	}
	for _, point := range daily {
		report.Daily = append(report.Daily, *point)
	}
	sort.Slice(report.Daily, func(i, j int) bool { return report.Daily[i].Date < report.Daily[j].Date })
	for _, row := range services {
		if row.Orders > 0 || row.RecognizedRevenueCents > 0 {
			report.ByService = append(report.ByService, *row)
		}
	}
	sort.Slice(report.ByService, func(i, j int) bool {
		if report.ByService[i].RecognizedRevenueCents == report.ByService[j].RecognizedRevenueCents {
			return report.ByService[i].Orders > report.ByService[j].Orders
		}
		return report.ByService[i].RecognizedRevenueCents > report.ByService[j].RecognizedRevenueCents
	})
	return report, nil
}

func (s *GORMStore) ListAuditLogs(ctx context.Context, hotelID uuid.UUID, filter AuditFilter) (AuditPage, error) {
	if filter.Limit < 1 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	query := s.db.WithContext(ctx).Model(&models.AuditLog{}).Where("hotel_id = ?", hotelID)
	if !filter.From.IsZero() {
		query = query.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		query = query.Where("created_at < ?", filter.To)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Outcome != "" {
		query = query.Where("outcome = ?", filter.Outcome)
	}
	page := AuditPage{Items: []models.AuditLog{}, Limit: filter.Limit, Offset: filter.Offset}
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	err := query.Order("created_at DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&page.Items).Error
	return page, err
}

var _ ReportingStore = (*GORMStore)(nil)
