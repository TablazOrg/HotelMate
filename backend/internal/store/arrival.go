package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrArrivalDisabled     = errors.New("online check-in is disabled")
	ErrInvitationExpired   = errors.New("check-in invitation expired")
	ErrInvitationRevoked   = errors.New("check-in invitation revoked")
	ErrArrivalIncomplete   = errors.New("arrival journey is incomplete")
	ErrTermsOutdated       = errors.New("registration terms changed")
	ErrEvidenceUnavailable = errors.New("arrival evidence is unavailable")
)

type ArrivalDetailsInput struct {
	ContactPhone       string
	ContactEmail       string
	Nationality        string
	ArrivalETA         *time.Time
	ArrivalMethod      string
	TransportDetails   string
	AccessibilityNeeds string
	SpecialRequests    string
	AnswersJSON        []byte
}

type ArrivalFilter struct {
	Status     models.ArrivalStatus
	RiskState  string
	OwnerID    *uuid.UUID
	Unassigned bool
	From       *time.Time
	To         *time.Time
}

type ArrivalAnalytics struct {
	From                   time.Time        `json:"from"`
	To                     time.Time        `json:"to"`
	Invitations            int64            `json:"invitations"`
	Opened                 int64            `json:"opened"`
	Started                int64            `json:"started"`
	Submitted              int64            `json:"submitted"`
	Approved               int64            `json:"approved"`
	NeedsChanges           int64            `json:"needsChanges"`
	RoomReady              int64            `json:"roomReady"`
	CheckedIn              int64            `json:"checkedIn"`
	TechnicalFailures      int64            `json:"technicalFailures"`
	Abandoned              int64            `json:"abandoned"`
	CompletionRate         float64          `json:"completionRate"`
	DocumentReworkRate     float64          `json:"documentReworkRate"`
	MedianCompletionMillis int64            `json:"medianCompletionMillis"`
	MedianReviewMillis     int64            `json:"medianReviewMillis"`
	StepEvents             map[string]int64 `json:"stepEvents"`
}

type ArrivalStore interface {
	GetArrivalSettings(context.Context, uuid.UUID) (models.ArrivalSettings, error)
	UpdateArrivalSettings(context.Context, uuid.UUID, models.ArrivalSettings) (models.ArrivalSettings, error)
	CreateCheckInInvitation(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, time.Time, time.Time) (models.CheckInInvitation, error)
	RevokeCheckInInvitation(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error
	RedeemCheckInInvitation(context.Context, string, string, *uuid.UUID, *uuid.UUID, time.Time) (models.CheckInInvitation, models.Stay, models.ArrivalJourney, models.ArrivalSettings, error)
	GetGuestArrival(context.Context, uuid.UUID, uuid.UUID) (models.ArrivalJourney, models.ArrivalSettings, error)
	SaveArrivalDetails(context.Context, uuid.UUID, uuid.UUID, ArrivalDetailsInput, time.Time) (models.ArrivalJourney, error)
	ReplaceArrivalCompanions(context.Context, uuid.UUID, uuid.UUID, []models.ArrivalCompanion, time.Time) (models.ArrivalJourney, error)
	AddArrivalDocument(context.Context, uuid.UUID, uuid.UUID, models.ArrivalDocument, time.Time) (models.ArrivalDocument, error)
	DeleteArrivalDocument(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (string, error)
	SaveArrivalSignature(context.Context, uuid.UUID, uuid.UUID, models.ArrivalJourney, time.Time) (models.ArrivalJourney, string, error)
	SubmitArrival(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (models.ArrivalJourney, error)
	CancelArrival(context.Context, uuid.UUID, uuid.UUID, time.Time) (models.ArrivalJourney, error)
	RecordArrivalEvent(context.Context, uuid.UUID, uuid.UUID, string, int, string, *uuid.UUID, int64, time.Time) error
	ListArrivals(context.Context, uuid.UUID, ArrivalFilter) ([]models.ArrivalJourney, error)
	AssignArrival(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (models.ArrivalJourney, error)
	ReviewArrival(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.ArrivalStatus, string, time.Time) (models.ArrivalJourney, error)
	TransitionArrival(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.ArrivalStatus, time.Time) (models.ArrivalJourney, error)
	GetArrivalDocument(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (models.ArrivalDocument, error)
	GetArrivalSignature(context.Context, uuid.UUID, uuid.UUID) (models.ArrivalJourney, error)
	ArrivalAnalytics(context.Context, uuid.UUID, time.Time, time.Time) (ArrivalAnalytics, error)
	ListExpiredArrivalEvidence(context.Context, time.Time, int) ([]models.ArrivalDocument, []models.ArrivalJourney, error)
	MarkArrivalDocumentDeleted(context.Context, uuid.UUID, time.Time) error
	MarkArrivalSignatureDeleted(context.Context, uuid.UUID, time.Time) error
}

func HashInvitationSecret(value string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(digest[:])
}

func (s *GORMStore) GetArrivalSettings(ctx context.Context, hotelID uuid.UUID) (models.ArrivalSettings, error) {
	var settings models.ArrivalSettings
	err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).First(&settings).Error
	return settings, err
}

func (s *GORMStore) UpdateArrivalSettings(ctx context.Context, hotelID uuid.UUID, input models.ArrivalSettings) (models.ArrivalSettings, error) {
	var settings models.ArrivalSettings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ?", hotelID).First(&settings).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"online_check_in_enabled":      input.OnlineCheckInEnabled,
			"digital_registration_enabled": input.DigitalRegistrationEnabled,
			"payment_step_enabled":         input.PaymentStepEnabled,
			"invitation_ttl_hours":         input.InvitationTTLHours,
			"terms_version":                input.TermsVersion,
			"terms_locale":                 input.TermsLocale,
			"terms_text":                   input.TermsText,
			"steps_json":                   input.StepsJSON,
		}
		return tx.Model(&settings).Updates(updates).Error
	})
	if err != nil {
		return models.ArrivalSettings{}, err
	}
	return s.GetArrivalSettings(ctx, hotelID)
}

func (s *GORMStore) CreateCheckInInvitation(ctx context.Context, hotelID, reservationID, actorID uuid.UUID, tokenHash, recoveryHash string, expiresAt, at time.Time) (models.CheckInInvitation, error) {
	var invitation models.CheckInInvitation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var settings models.ArrivalSettings
		if err := tx.Where("hotel_id = ?", hotelID).First(&settings).Error; err != nil {
			return err
		}
		if !settings.OnlineCheckInEnabled {
			return ErrArrivalDisabled
		}
		var reservation models.Reservation
		if err := tx.Where("hotel_id = ? AND id = ?", hotelID, reservationID).First(&reservation).Error; err != nil {
			return err
		}
		if reservation.Status != models.ReservationConfirmed || reservation.RoomID == nil {
			return ErrInvalidTransition
		}
		invitation = models.CheckInInvitation{
			HotelID: hotelID, ReservationID: reservationID, CreatedByID: actorID,
			TokenHash: tokenHash, RecoveryCodeHash: recoveryHash, ExpiresAt: expiresAt,
		}
		invitation.CreatedAt = at
		if err := tx.Create(&invitation).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, EventType: "invitation", ActorType: "staff", ActorID: &actorID}).Error
	})
	return invitation, err
}

func (s *GORMStore) RevokeCheckInInvitation(ctx context.Context, hotelID, invitationID, actorID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&models.CheckInInvitation{}).
		Where("hotel_id = ? AND id = ? AND revoked_at IS NULL", hotelID, invitationID).
		Updates(map[string]any{"revoked_at": at, "revoked_by_id": actorID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *GORMStore) RedeemCheckInInvitation(ctx context.Context, tokenHash, recoveryHash string, reservationID, hotelID *uuid.UUID, at time.Time) (models.CheckInInvitation, models.Stay, models.ArrivalJourney, models.ArrivalSettings, error) {
	var invitation models.CheckInInvitation
	var stay models.Stay
	var journey models.ArrivalJourney
	var settings models.ArrivalSettings
	journeyExpired := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.CheckInInvitation{})
		if tokenHash != "" {
			query = query.Where("token_hash = ?", tokenHash)
		} else {
			query = query.Where("recovery_code_hash = ?", recoveryHash)
		}
		if reservationID != nil && hotelID != nil {
			query = query.Where("reservation_id = ? AND hotel_id = ?", *reservationID, *hotelID)
		}
		if err := query.First(&invitation).Error; err != nil {
			return err
		}
		if invitation.RevokedAt != nil {
			return ErrInvitationRevoked
		}
		if !at.Before(invitation.ExpiresAt) {
			return ErrInvitationExpired
		}
		if err := tx.Where("hotel_id = ?", invitation.HotelID).First(&settings).Error; err != nil {
			return err
		}
		if !settings.OnlineCheckInEnabled {
			return ErrArrivalDisabled
		}
		var reservation models.Reservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", invitation.HotelID, invitation.ReservationID).First(&reservation).Error; err != nil {
			return err
		}
		if reservation.Status != models.ReservationConfirmed || reservation.RoomID == nil {
			return ErrInvalidTransition
		}
		err := tx.Where("hotel_id = ? AND reservation_id = ?", invitation.HotelID, reservation.ID).First(&stay).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			stay = models.Stay{HotelID: invitation.HotelID, GuestID: reservation.GuestID, RoomID: *reservation.RoomID, ReservationID: &reservation.ID, Status: models.StayPreArrival}
			if err := tx.Create(&stay).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		err = tx.Where("hotel_id = ? AND reservation_id = ?", invitation.HotelID, reservation.ID).First(&journey).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			expiresAt := reservation.DepartureDate.Add(24 * time.Hour)
			journey = models.ArrivalJourney{
				HotelID: invitation.HotelID, ReservationID: reservation.ID, StayID: stay.ID,
				Status: models.ArrivalDraft, CurrentStep: 1, RiskState: "incomplete", ExpiresAt: expiresAt,
				ContactPhone: "", AnswersJSON: []byte(`{}`),
			}
			if err := tx.Create(&journey).Error; err != nil {
				return err
			}
			payment := models.ArrivalPaymentStep{HotelID: invitation.HotelID, JourneyID: journey.ID, Required: false, Status: "not_required", Capability: "deferred_to_m11"}
			if settings.PaymentStepEnabled {
				payment.Status = "provider_not_configured"
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if !at.Before(journey.ExpiresAt) && journey.Status != models.ArrivalCheckedIn && journey.Status != models.ArrivalCancelled && journey.Status != models.ArrivalExpired {
			journey.Status = models.ArrivalExpired
			if err := tx.Model(&journey).Update("status", models.ArrivalExpired).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: invitation.HotelID, JourneyID: journey.ID, EventType: "expire", ActorType: "system"}).Error; err != nil {
				return err
			}
			journeyExpired = true
			return nil
		}
		if journey.Status == models.ArrivalCancelled || journey.Status == models.ArrivalExpired {
			return ErrInvalidTransition
		}
		if stay.Status == models.StayActive && journey.Status != models.ArrivalCheckedIn {
			journey.Status, journey.CheckedInAt = models.ArrivalCheckedIn, stay.CheckInAt
			if err := tx.Model(&journey).Updates(map[string]any{"status": journey.Status, "checked_in_at": journey.CheckedInAt}).Error; err != nil {
				return err
			}
		}
		updates := map[string]any{"last_opened_at": at}
		if invitation.OpenedAt == nil {
			updates["opened_at"] = at
			invitation.OpenedAt = &at
		}
		invitation.LastOpenedAt = &at
		if err := tx.Model(&invitation).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: invitation.HotelID, JourneyID: journey.ID, EventType: "open", ActorType: "guest", ActorID: &stay.GuestID}).Error
	})
	if err != nil {
		return models.CheckInInvitation{}, models.Stay{}, models.ArrivalJourney{}, models.ArrivalSettings{}, err
	}
	if journeyExpired {
		return models.CheckInInvitation{}, models.Stay{}, models.ArrivalJourney{}, models.ArrivalSettings{}, ErrInvitationExpired
	}
	loadedStay, err := s.loadStay(ctx, invitation.HotelID, stay.ID)
	if err != nil {
		return models.CheckInInvitation{}, models.Stay{}, models.ArrivalJourney{}, models.ArrivalSettings{}, err
	}
	loadedJourney, err := s.loadArrival(ctx, invitation.HotelID, journey.ID)
	return invitation, loadedStay, loadedJourney, settings, err
}

func (s *GORMStore) GetGuestArrival(ctx context.Context, hotelID, stayID uuid.UUID) (models.ArrivalJourney, models.ArrivalSettings, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, time.Now().UTC()); err != nil {
		return models.ArrivalJourney{}, models.ArrivalSettings{}, err
	}
	settings, err := s.GetArrivalSettings(ctx, hotelID)
	if err != nil {
		return models.ArrivalJourney{}, models.ArrivalSettings{}, err
	}
	if !settings.OnlineCheckInEnabled {
		return models.ArrivalJourney{}, settings, ErrArrivalDisabled
	}
	var journey models.ArrivalJourney
	err = s.arrivalPreloads(s.db.WithContext(ctx)).Where("arrival_journeys.hotel_id = ? AND arrival_journeys.stay_id = ?", hotelID, stayID).First(&journey).Error
	return journey, settings, err
}

func (s *GORMStore) SaveArrivalDetails(ctx context.Context, hotelID, stayID uuid.UUID, input ArrivalDetailsInput, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		complete := strings.TrimSpace(input.ContactPhone) != "" && input.ArrivalETA != nil && strings.TrimSpace(input.ArrivalMethod) != ""
		currentStep := journey.CurrentStep
		var completedAt *time.Time
		if complete {
			completedAt = &at
			if currentStep < 2 {
				currentStep = 2
			}
		} else {
			currentStep = 1
		}
		updates := map[string]any{
			"contact_phone": input.ContactPhone, "contact_email": input.ContactEmail, "nationality": input.Nationality,
			"arrival_eta": input.ArrivalETA, "arrival_method": input.ArrivalMethod, "transport_details": input.TransportDetails,
			"accessibility_needs": input.AccessibilityNeeds, "special_requests": input.SpecialRequests,
			"answers_json": input.AnswersJSON, "current_step": currentStep, "details_completed_at": completedAt,
		}
		if err := tx.Model(&journey).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "save", Step: 1, ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.recalculateArrival(ctx, hotelID, journey.ID)
}

func (s *GORMStore) ReplaceArrivalCompanions(ctx context.Context, hotelID, stayID uuid.UUID, companions []models.ArrivalCompanion, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		var documentCount int64
		if err := tx.Model(&models.ArrivalDocument{}).Where("hotel_id = ? AND journey_id = ? AND companion_id IS NOT NULL AND deleted_at_storage IS NULL", hotelID, journey.ID).Count(&documentCount).Error; err != nil {
			return err
		}
		if documentCount > 0 {
			return ErrInvalidTransition
		}
		if err := tx.Where("hotel_id = ? AND journey_id = ?", hotelID, journey.ID).Delete(&models.ArrivalCompanion{}).Error; err != nil {
			return err
		}
		for index := range companions {
			companions[index].HotelID = hotelID
			companions[index].JourneyID = journey.ID
		}
		if len(companions) > 0 {
			if err := tx.Create(&companions).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "save", Step: 1, ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.recalculateArrival(ctx, hotelID, journey.ID)
}

func (s *GORMStore) AddArrivalDocument(ctx context.Context, hotelID, stayID uuid.UUID, document models.ArrivalDocument, at time.Time) (models.ArrivalDocument, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalDocument{}, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journey models.ArrivalJourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		if document.CompanionID != nil {
			var count int64
			if err := tx.Model(&models.ArrivalCompanion{}).Where("hotel_id = ? AND journey_id = ? AND id = ?", hotelID, journey.ID, *document.CompanionID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		document.HotelID, document.JourneyID = hotelID, journey.ID
		document.CreatedAt = at
		if err := tx.Create(&document).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "save", Step: 2, ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalDocument{}, err
	}
	_, _ = s.recalculateArrival(ctx, hotelID, document.JourneyID)
	return document, nil
}

func (s *GORMStore) DeleteArrivalDocument(ctx context.Context, hotelID, stayID, documentID uuid.UUID, at time.Time) (string, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return "", err
	}
	var key string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journey models.ArrivalJourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		var document models.ArrivalDocument
		if err := tx.Where("hotel_id = ? AND journey_id = ? AND id = ? AND deleted_at_storage IS NULL", hotelID, journey.ID, documentID).First(&document).Error; err != nil {
			return err
		}
		key = document.StorageKey
		return tx.Model(&document).Update("deleted_at_storage", at).Error
	})
	return key, err
}

func (s *GORMStore) SaveArrivalSignature(ctx context.Context, hotelID, stayID uuid.UUID, signature models.ArrivalJourney, at time.Time) (models.ArrivalJourney, string, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, "", err
	}
	var journey models.ArrivalJourney
	var oldKey string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		var settings models.ArrivalSettings
		if err := tx.Where("hotel_id = ?", hotelID).First(&settings).Error; err != nil {
			return err
		}
		if signature.TermsVersion != settings.TermsVersion || signature.TermsLocale != settings.TermsLocale {
			return ErrTermsOutdated
		}
		oldKey = journey.SignatureStorageKey
		updates := map[string]any{
			"terms_version": signature.TermsVersion, "terms_locale": signature.TermsLocale,
			"signer_name": signature.SignerName, "consent_at": at,
			"signature_storage_key": signature.SignatureStorageKey, "signature_media_type": signature.SignatureMediaType,
			"signature_size": signature.SignatureSize, "signature_sha256": signature.SignatureSHA256,
			"signature_deleted_at": nil, "current_step": 3, "documents_completed_at": at,
		}
		if err := tx.Model(&journey).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "save", Step: 2, ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, "", err
	}
	loaded, err := s.recalculateArrival(ctx, hotelID, journey.ID)
	return loaded, oldKey, err
}

func (s *GORMStore) SubmitArrival(ctx context.Context, hotelID, stayID uuid.UUID, idempotencyKey string, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if journey.LastIdempotencyKey == idempotencyKey && (journey.Status == models.ArrivalSubmitted || journey.Status == models.ArrivalApproved || journey.Status == models.ArrivalPending || journey.Status == models.ArrivalRoomReady) {
			return nil
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		if err := s.validateArrivalTx(tx, &journey); err != nil {
			return err
		}
		eventType := "submit"
		if journey.Status == models.ArrivalNeedsChanges {
			eventType = "resubmit"
		}
		updates := map[string]any{
			"status": models.ArrivalSubmitted, "current_step": 3, "completeness_score": 100,
			"risk_state": "manual_review", "submitted_at": at, "last_idempotency_key": idempotencyKey,
		}
		if err := tx.Model(&journey).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: eventType, Step: 3, ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.loadArrival(ctx, hotelID, journey.ID)
}

func (s *GORMStore) CancelArrival(ctx context.Context, hotelID, stayID uuid.UUID, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND stay_id = ?", hotelID, stayID).First(&journey).Error; err != nil {
			return err
		}
		if !arrivalGuestEditable(journey.Status) {
			return ErrInvalidTransition
		}
		if err := tx.Model(&journey).Updates(map[string]any{"status": models.ArrivalCancelled, "cancelled_at": at}).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "cancel", ActorType: "guest"}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.loadArrival(ctx, hotelID, journey.ID)
}

func (s *GORMStore) RecordArrivalEvent(ctx context.Context, hotelID, journeyID uuid.UUID, eventType string, step int, actorType string, actorID *uuid.UUID, durationMS int64, at time.Time) error {
	return s.db.WithContext(ctx).Create(&models.ArrivalEvent{
		HotelID: hotelID, JourneyID: journeyID, EventType: eventType, Step: step,
		ActorType: actorType, ActorID: actorID, DurationMS: durationMS, BaseModel: models.BaseModel{CreatedAt: at},
	}).Error
}

func (s *GORMStore) ListArrivals(ctx context.Context, hotelID uuid.UUID, filter ArrivalFilter) ([]models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, time.Now().UTC()); err != nil {
		return nil, err
	}
	query := s.arrivalPreloads(s.db.WithContext(ctx)).Where("arrival_journeys.hotel_id = ?", hotelID)
	if filter.Status != "" {
		query = query.Where("arrival_journeys.status = ?", filter.Status)
	}
	if filter.RiskState != "" {
		query = query.Where("arrival_journeys.risk_state = ?", filter.RiskState)
	}
	if filter.OwnerID != nil {
		query = query.Where("arrival_journeys.review_owner_id = ?", *filter.OwnerID)
	}
	if filter.Unassigned {
		query = query.Where("arrival_journeys.review_owner_id IS NULL")
	}
	if filter.From != nil {
		query = query.Where("reservations.arrival_date >= ?", *filter.From).Joins("JOIN reservations ON reservations.id = arrival_journeys.reservation_id")
	}
	if filter.To != nil {
		if filter.From == nil {
			query = query.Joins("JOIN reservations ON reservations.id = arrival_journeys.reservation_id")
		}
		query = query.Where("reservations.arrival_date <= ?", *filter.To)
	}
	var journeys []models.ArrivalJourney
	err := query.Order("arrival_journeys.submitted_at ASC NULLS LAST, arrival_journeys.created_at ASC").Find(&journeys).Error
	return journeys, err
}

func (s *GORMStore) AssignArrival(ctx context.Context, hotelID, journeyID, actorID, ownerID uuid.UUID, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	var staff models.StaffUser
	if err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ? AND is_active = ?", hotelID, ownerID, true).First(&staff).Error; err != nil {
		return models.ArrivalJourney{}, err
	}
	result := s.db.WithContext(ctx).Model(&models.ArrivalJourney{}).Where("hotel_id = ? AND id = ? AND status NOT IN ?", hotelID, journeyID, []models.ArrivalStatus{models.ArrivalExpired, models.ArrivalCancelled, models.ArrivalCheckedIn}).Update("review_owner_id", ownerID)
	if result.Error != nil {
		return models.ArrivalJourney{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.ArrivalJourney{}, ErrInvalidTransition
	}
	_ = s.RecordArrivalEvent(ctx, hotelID, journeyID, "review", 0, "staff", &actorID, 0, at)
	return s.loadArrival(ctx, hotelID, journeyID)
}

func (s *GORMStore) ReviewArrival(ctx context.Context, hotelID, journeyID, actorID uuid.UUID, status models.ArrivalStatus, reason string, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journey models.ArrivalJourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, journeyID).First(&journey).Error; err != nil {
			return err
		}
		if journey.Status != models.ArrivalSubmitted || (status != models.ArrivalApproved && status != models.ArrivalNeedsChanges) {
			return ErrInvalidTransition
		}
		updates := map[string]any{"status": status, "reviewed_at": at, "reviewed_by_id": actorID, "review_owner_id": actorID}
		eventType := "approve"
		if status == models.ArrivalApproved {
			updates["approved_at"] = at
			updates["risk_state"] = "verified"
			updates["needs_changes_reason"] = ""
		} else {
			if strings.TrimSpace(reason) == "" {
				return ErrArrivalIncomplete
			}
			updates["risk_state"] = "needs_changes"
			updates["needs_changes_reason"] = reason
			eventType = "needs_changes"
		}
		if err := tx.Model(&journey).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journeyID, EventType: eventType, ActorType: "staff", ActorID: &actorID}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.loadArrival(ctx, hotelID, journeyID)
}

func (s *GORMStore) TransitionArrival(ctx context.Context, hotelID, journeyID, actorID uuid.UUID, status models.ArrivalStatus, at time.Time) (models.ArrivalJourney, error) {
	if err := s.expireArrivalJourneys(ctx, hotelID, at); err != nil {
		return models.ArrivalJourney{}, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journey models.ArrivalJourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, journeyID).First(&journey).Error; err != nil {
			return err
		}
		allowed := (journey.Status == models.ArrivalApproved && status == models.ArrivalPending) ||
			((journey.Status == models.ArrivalApproved || journey.Status == models.ArrivalPending) && status == models.ArrivalRoomReady)
		if !allowed {
			return ErrInvalidTransition
		}
		updates := map[string]any{"status": status}
		eventType := "arrival_pending"
		if status == models.ArrivalPending {
			updates["arrival_pending_at"] = at
		} else {
			var room models.Room
			if err := tx.Joins("JOIN stays ON stays.room_id = rooms.id").Where("stays.id = ? AND rooms.hotel_id = ?", journey.StayID, hotelID).First(&room).Error; err != nil {
				return err
			}
			if room.Status != models.RoomStatusAvailable {
				return ErrRoomUnavailable
			}
			updates["room_ready_at"] = at
			eventType = "room_ready"
		}
		if err := tx.Model(&journey).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journeyID, EventType: eventType, ActorType: "staff", ActorID: &actorID}).Error
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.loadArrival(ctx, hotelID, journeyID)
}

func (s *GORMStore) GetArrivalDocument(ctx context.Context, hotelID, journeyID, documentID uuid.UUID) (models.ArrivalDocument, error) {
	var document models.ArrivalDocument
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND journey_id = ? AND id = ?", hotelID, journeyID, documentID).First(&document).Error
	if err == nil && (document.DeletedAtStorage != nil || !time.Now().UTC().Before(document.RetentionUntil)) {
		return models.ArrivalDocument{}, ErrEvidenceUnavailable
	}
	return document, err
}

func (s *GORMStore) GetArrivalSignature(ctx context.Context, hotelID, journeyID uuid.UUID) (models.ArrivalJourney, error) {
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, journeyID).First(&journey).Error
	if err == nil && (journey.SignatureStorageKey == "" || journey.SignatureDeletedAt != nil || !time.Now().UTC().Before(journey.ExpiresAt)) {
		return models.ArrivalJourney{}, ErrEvidenceUnavailable
	}
	return journey, err
}

func (s *GORMStore) ArrivalAnalytics(ctx context.Context, hotelID uuid.UUID, from, to time.Time) (ArrivalAnalytics, error) {
	result := ArrivalAnalytics{From: from, To: to, StepEvents: map[string]int64{}}
	rows, err := s.db.WithContext(ctx).Model(&models.ArrivalEvent{}).
		Select("event_type, step, count(*) AS total").
		Where("hotel_id = ? AND created_at >= ? AND created_at < ?", hotelID, from, to).
		Group("event_type, step").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var eventType string
		var step int
		var total int64
		if err := rows.Scan(&eventType, &step, &total); err != nil {
			return result, err
		}
		result.StepEvents[fmt.Sprintf("%s:%d", eventType, step)] = total
		switch eventType {
		case "invitation":
			result.Invitations += total
		case "open":
			result.Opened += total
		case "start":
			result.Started += total
		case "submit", "resubmit":
			result.Submitted += total
		case "approve":
			result.Approved += total
		case "needs_changes":
			result.NeedsChanges += total
		case "room_ready":
			result.RoomReady += total
		case "physical_arrival":
			result.CheckedIn += total
		case "error":
			result.TechnicalFailures += total
		case "abandon":
			result.Abandoned += total
		}
	}
	if result.Opened > 0 {
		result.CompletionRate = float64(result.Submitted) / float64(result.Opened)
	}
	if result.Submitted > 0 {
		result.DocumentReworkRate = float64(result.NeedsChanges) / float64(result.Submitted)
	}
	var completion []int64
	if err := s.db.WithContext(ctx).Raw(`
		SELECT (EXTRACT(EPOCH FROM (submitted_at - created_at)) * 1000)::bigint
		FROM arrival_journeys
		WHERE hotel_id = ? AND submitted_at >= ? AND submitted_at < ? AND submitted_at IS NOT NULL
		ORDER BY 1`, hotelID, from, to).Scan(&completion).Error; err != nil {
		return result, err
	}
	result.MedianCompletionMillis = medianInt64(completion)
	var review []int64
	if err := s.db.WithContext(ctx).Raw(`
		SELECT (EXTRACT(EPOCH FROM (reviewed_at - submitted_at)) * 1000)::bigint
		FROM arrival_journeys
		WHERE hotel_id = ? AND reviewed_at >= ? AND reviewed_at < ? AND submitted_at IS NOT NULL AND reviewed_at IS NOT NULL
		ORDER BY 1`, hotelID, from, to).Scan(&review).Error; err != nil {
		return result, err
	}
	result.MedianReviewMillis = medianInt64(review)
	return result, nil
}

func (s *GORMStore) ListExpiredArrivalEvidence(ctx context.Context, before time.Time, limit int) ([]models.ArrivalDocument, []models.ArrivalJourney, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var documents []models.ArrivalDocument
	if err := s.db.WithContext(ctx).Where("retention_until <= ? AND deleted_at_storage IS NULL", before).Order("retention_until ASC").Limit(limit).Find(&documents).Error; err != nil {
		return nil, nil, err
	}
	var signatures []models.ArrivalJourney
	if err := s.db.WithContext(ctx).Where("expires_at <= ? AND signature_storage_key <> '' AND signature_deleted_at IS NULL", before).Order("expires_at ASC").Limit(limit).Find(&signatures).Error; err != nil {
		return nil, nil, err
	}
	return documents, signatures, nil
}

func (s *GORMStore) MarkArrivalDocumentDeleted(ctx context.Context, documentID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&models.ArrivalDocument{}).Where("id = ? AND deleted_at_storage IS NULL", documentID).Update("deleted_at_storage", at)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *GORMStore) MarkArrivalSignatureDeleted(ctx context.Context, journeyID uuid.UUID, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&models.ArrivalJourney{}).Where("id = ? AND signature_deleted_at IS NULL", journeyID).Update("signature_deleted_at", at)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *GORMStore) expireArrivalJourneys(ctx context.Context, hotelID uuid.UUID, at time.Time) error {
	activeStatuses := []models.ArrivalStatus{
		models.ArrivalDraft, models.ArrivalSubmitted, models.ArrivalNeedsChanges,
		models.ArrivalApproved, models.ArrivalPending, models.ArrivalRoomReady,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journeys []models.ArrivalJourney
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "hotel_id").
			Where("hotel_id = ? AND expires_at <= ? AND status IN ?", hotelID, at, activeStatuses).
			Find(&journeys).Error; err != nil {
			return err
		}
		for _, journey := range journeys {
			if err := tx.Model(&models.ArrivalJourney{}).Where("hotel_id = ? AND id = ?", hotelID, journey.ID).Update("status", models.ArrivalExpired).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.ArrivalEvent{BaseModel: models.BaseModel{CreatedAt: at}, HotelID: hotelID, JourneyID: journey.ID, EventType: "expire", ActorType: "system"}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GORMStore) validateArrivalTx(tx *gorm.DB, journey *models.ArrivalJourney) error {
	var settings models.ArrivalSettings
	if err := tx.Where("hotel_id = ?", journey.HotelID).First(&settings).Error; err != nil {
		return err
	}
	if !settings.OnlineCheckInEnabled || !settings.DigitalRegistrationEnabled {
		return ErrArrivalDisabled
	}
	if strings.TrimSpace(journey.ContactPhone) == "" || journey.ArrivalETA == nil || strings.TrimSpace(journey.ArrivalMethod) == "" {
		return ErrArrivalIncomplete
	}
	if journey.SignatureStorageKey == "" || journey.SignatureDeletedAt != nil || journey.ConsentAt == nil || strings.TrimSpace(journey.SignerName) == "" {
		return ErrArrivalIncomplete
	}
	if journey.TermsVersion != settings.TermsVersion || journey.TermsLocale != settings.TermsLocale {
		return ErrTermsOutdated
	}
	var guestDocuments int64
	if err := tx.Model(&models.ArrivalDocument{}).Where("hotel_id = ? AND journey_id = ? AND companion_id IS NULL AND deleted_at_storage IS NULL", journey.HotelID, journey.ID).Count(&guestDocuments).Error; err != nil {
		return err
	}
	if guestDocuments == 0 {
		return ErrArrivalIncomplete
	}
	var companions []models.ArrivalCompanion
	if err := tx.Where("hotel_id = ? AND journey_id = ? AND document_required = ?", journey.HotelID, journey.ID, true).Find(&companions).Error; err != nil {
		return err
	}
	for _, companion := range companions {
		var count int64
		if err := tx.Model(&models.ArrivalDocument{}).Where("hotel_id = ? AND journey_id = ? AND companion_id = ? AND deleted_at_storage IS NULL", journey.HotelID, journey.ID, companion.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrArrivalIncomplete
		}
	}
	return nil
}

func (s *GORMStore) recalculateArrival(ctx context.Context, hotelID, journeyID uuid.UUID) (models.ArrivalJourney, error) {
	var journey models.ArrivalJourney
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, journeyID).First(&journey).Error; err != nil {
			return err
		}
		score := 0
		if journey.ContactPhone != "" && journey.ArrivalETA != nil && journey.ArrivalMethod != "" {
			score += 35
		}
		var documents int64
		if err := tx.Model(&models.ArrivalDocument{}).Where("hotel_id = ? AND journey_id = ? AND deleted_at_storage IS NULL", hotelID, journeyID).Count(&documents).Error; err != nil {
			return err
		}
		if documents > 0 {
			score += 35
		}
		if journey.SignatureStorageKey != "" && journey.SignatureDeletedAt == nil && journey.ConsentAt != nil {
			score += 30
		}
		if err := tx.Model(&journey).Update("completeness_score", score).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.ArrivalJourney{}, err
	}
	return s.loadArrival(ctx, hotelID, journeyID)
}

func (s *GORMStore) loadArrival(ctx context.Context, hotelID, journeyID uuid.UUID) (models.ArrivalJourney, error) {
	var journey models.ArrivalJourney
	err := s.arrivalPreloads(s.db.WithContext(ctx)).Where("arrival_journeys.hotel_id = ? AND arrival_journeys.id = ?", hotelID, journeyID).First(&journey).Error
	return journey, err
}

func (s *GORMStore) arrivalPreloads(query *gorm.DB) *gorm.DB {
	return query.Preload("Reservation.Guest").Preload("Reservation.Room").
		Preload("Stay.Guest").Preload("Stay.Room").Preload("Stay.Hotel").Preload("Stay.Reservation.Guest").Preload("Stay.Reservation.Room").
		Preload("Documents", "deleted_at_storage IS NULL").Preload("Companions").Preload("PaymentStep")
}

func arrivalGuestEditable(status models.ArrivalStatus) bool {
	return status == models.ArrivalDraft || status == models.ArrivalNeedsChanges
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

var _ ArrivalStore = (*GORMStore)(nil)
