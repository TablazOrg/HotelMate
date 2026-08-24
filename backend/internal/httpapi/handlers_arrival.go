package httpapi

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/documents"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

var arrivalRoles = []models.StaffRole{
	models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations, models.StaffRoleReception,
}

func (s *Server) registerArrivalRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/check-in/exchange", s.limit(s.authLimiter, http.HandlerFunc(s.exchangeCheckInInvitation)))

	mux.Handle("GET /api/v1/guest/arrival", s.require(auth.ActorGuest)(http.HandlerFunc(s.getGuestArrival)))
	mux.Handle("PATCH /api/v1/guest/arrival/details", s.require(auth.ActorGuest)(http.HandlerFunc(s.saveGuestArrivalDetails)))
	mux.Handle("PUT /api/v1/guest/arrival/companions", s.require(auth.ActorGuest)(http.HandlerFunc(s.replaceGuestArrivalCompanions)))
	mux.Handle("POST /api/v1/guest/arrival/documents", s.require(auth.ActorGuest)(http.HandlerFunc(s.uploadGuestArrivalDocument)))
	mux.Handle("DELETE /api/v1/guest/arrival/documents/{id}", s.require(auth.ActorGuest)(http.HandlerFunc(s.deleteGuestArrivalDocument)))
	mux.Handle("POST /api/v1/guest/arrival/signature", s.require(auth.ActorGuest)(http.HandlerFunc(s.saveGuestArrivalSignature)))
	mux.Handle("POST /api/v1/guest/arrival/submit", s.require(auth.ActorGuest)(http.HandlerFunc(s.submitGuestArrival)))
	mux.Handle("POST /api/v1/guest/arrival/cancel", s.require(auth.ActorGuest)(http.HandlerFunc(s.cancelGuestArrival)))
	mux.Handle("POST /api/v1/guest/arrival/events", s.require(auth.ActorGuest)(http.HandlerFunc(s.recordGuestArrivalEvent)))

	mux.Handle("GET /api/v1/staff/arrival-settings", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.getArrivalSettings)))
	mux.Handle("PATCH /api/v1/staff/arrival-settings", s.require(auth.ActorStaff, models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin)(http.HandlerFunc(s.updateArrivalSettings)))
	mux.Handle("POST /api/v1/staff/reservations/{id}/check-in-invitations", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.createCheckInInvitation)))
	mux.Handle("POST /api/v1/staff/check-in-invitations/{id}/revoke", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.revokeCheckInInvitation)))
	mux.Handle("GET /api/v1/staff/arrivals", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.listStaffArrivals)))
	mux.Handle("GET /api/v1/staff/arrivals/analytics", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.arrivalAnalytics)))
	mux.Handle("POST /api/v1/staff/arrivals/remind", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.bulkRemindArrivals)))
	mux.Handle("POST /api/v1/staff/arrivals/{id}/assign", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.assignArrival)))
	mux.Handle("POST /api/v1/staff/arrivals/{id}/review", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.reviewArrival)))
	mux.Handle("POST /api/v1/staff/arrivals/{id}/status", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.transitionArrival)))
	mux.Handle("POST /api/v1/staff/arrivals/{id}/remind", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.remindArrival)))
	mux.Handle("GET /api/v1/staff/arrivals/{id}/documents/{documentId}", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.downloadArrivalDocument)))
	mux.Handle("GET /api/v1/staff/arrivals/{id}/signature", s.require(auth.ActorStaff, arrivalRoles...)(http.HandlerFunc(s.downloadArrivalSignature)))
}

func (s *Server) requireArrival(w http.ResponseWriter) bool {
	if s.arrival == nil || s.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "arrival_unavailable", "سرویس آمادگی ورود در دسترس نیست")
		return false
	}
	return true
}

type createInvitationRequest struct {
	TTLHours int `json:"ttlHours"`
}

func (s *Server) createCheckInInvitation(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	reservationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_reservation", "شناسه رزرو معتبر نیست")
		return
	}
	var input createInvitationRequest
	if r.ContentLength > 0 && !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.arrival.GetArrivalSettings(r.Context(), staff.HotelID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	if input.TTLHours == 0 {
		input.TTLHours = settings.InvitationTTLHours
	}
	if input.TTLHours < 1 || input.TTLHours > 720 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اعتبار دعوت باید بین یک ساعت و ۳۰ روز باشد")
		return
	}
	invitationToken, expiresAt, err := s.tokens.IssueCheckInInvitation(reservationID, staff.HotelID, time.Duration(input.TTLHours)*time.Hour)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	recoveryCode, err := randomRecoveryCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد دعوت انجام نشد")
		return
	}
	now := time.Now().UTC()
	invitation, err := s.arrival.CreateCheckInInvitation(r.Context(), staff.HotelID, reservationID, staff.ID, store.HashInvitationSecret(invitationToken), store.HashInvitationSecret(recoveryCode), expiresAt, now)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.invitation.create", models.AuditOutcomeSuccess, map[string]any{"invitationId": invitation.ID, "reservationId": reservationID, "expiresAt": expiresAt})
	recoveryLink := checkInBaseURL(r) + "/check-in#recovery=" + recoveryCode
	qrPNG, _ := qrcode.Encode(recoveryLink, qrcode.Medium, 256)
	qrDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrPNG)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": map[string]any{"id": invitation.ID, "expiresAt": expiresAt, "invitationToken": invitationToken, "recoveryCode": recoveryCode, "link": "/check-in#invite=" + invitationToken, "recoveryLink": recoveryLink, "qrDataUrl": qrDataURL},
	})
}

func (s *Server) revokeCheckInInvitation(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_invitation", "شناسه دعوت معتبر نیست")
		return
	}
	if err := s.arrival.RevokeCheckInInvitation(r.Context(), staff.HotelID, id, staff.ID, time.Now().UTC()); err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.invitation.revoke", models.AuditOutcomeSuccess, map[string]any{"invitationId": id})
	w.WriteHeader(http.StatusNoContent)
}

type exchangeInvitationRequest struct {
	InvitationToken string `json:"invitationToken"`
	RecoveryCode    string `json:"recoveryCode"`
}

func (s *Server) exchangeCheckInInvitation(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	var input exchangeInvitationRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.InvitationToken = strings.TrimSpace(input.InvitationToken)
	input.RecoveryCode = strings.ToUpper(strings.TrimSpace(input.RecoveryCode))
	if (input.InvitationToken == "") == (input.RecoveryCode == "") {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "فقط یکی از دعوت یا کد بازیابی را وارد کنید")
		return
	}
	var reservationID, hotelID *uuid.UUID
	if input.InvitationToken != "" {
		claims, err := s.tokens.ParseCheckInInvitation(input.InvitationToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_invitation", "دعوت معتبر نیست یا منقضی شده است")
			return
		}
		reservation, err := claims.SubjectID()
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_invitation", "دعوت معتبر نیست")
			return
		}
		hotel, err := claims.TenantID()
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_invitation", "دعوت معتبر نیست")
			return
		}
		reservationID, hotelID = &reservation, &hotel
	}
	tokenHash, recoveryHash := "", ""
	if input.InvitationToken != "" {
		tokenHash = store.HashInvitationSecret(input.InvitationToken)
	} else {
		recoveryHash = store.HashInvitationSecret(input.RecoveryCode)
	}
	invitation, stay, journey, settings, err := s.arrival.RedeemCheckInInvitation(
		r.Context(), tokenHash, recoveryHash, reservationID, hotelID, time.Now().UTC(),
	)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	token, expiresAt, err := s.tokens.IssueGuest(stay)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ایجاد نشست انجام نشد")
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.invitation.exchange", models.AuditOutcomeSuccess, map[string]any{"invitationId": invitation.ID, "journeyId": journey.ID})
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token, "tokenType": "Bearer", "expiresAt": expiresAt, "actorType": "guest",
		"stay": toStayView(stay), "arrival": toArrivalView(journey), "settings": toArrivalSettingsView(settings),
	})
}

func (s *Server) getGuestArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	journey, settings, err := s.arrival.GetGuestArrival(r.Context(), stay.HotelID, stay.ID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey), "settings": toArrivalSettingsView(settings)})
}

type saveArrivalDetailsRequest struct {
	ContactPhone       string          `json:"contactPhone"`
	ContactEmail       string          `json:"contactEmail"`
	Nationality        string          `json:"nationality"`
	ArrivalETA         string          `json:"arrivalEta"`
	ArrivalMethod      string          `json:"arrivalMethod"`
	TransportDetails   string          `json:"transportDetails"`
	AccessibilityNeeds string          `json:"accessibilityNeeds"`
	SpecialRequests    string          `json:"specialRequests"`
	Answers            json.RawMessage `json:"answers"`
}

func (s *Server) saveGuestArrivalDetails(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	var input saveArrivalDetailsRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.ContactEmail = strings.ToLower(strings.TrimSpace(input.ContactEmail))
	input.Nationality = strings.TrimSpace(input.Nationality)
	input.ArrivalMethod = strings.TrimSpace(input.ArrivalMethod)
	allowedMethod := map[string]bool{"car": true, "taxi": true, "flight": true, "train": true, "bus": true, "walk": true, "other": true}
	var eta *time.Time
	if strings.TrimSpace(input.ArrivalETA) != "" {
		parsed, err := time.Parse(time.RFC3339, input.ArrivalETA)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "زمان ورود معتبر نیست")
			return
		}
		parsed = parsed.UTC()
		eta = &parsed
	}
	if (input.ContactPhone != "" && (len(input.ContactPhone) < 6 || len(input.ContactPhone) > 40)) || (input.ContactEmail != "" && !validEmail(input.ContactEmail)) || (input.ArrivalMethod != "" && !allowedMethod[input.ArrivalMethod]) || len(input.TransportDetails) > 500 || len(input.AccessibilityNeeds) > 1000 || len(input.SpecialRequests) > 1000 || len(input.Answers) > 16*1024 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات ورود معتبر یا کامل نیست")
		return
	}
	if eta != nil && stay.Reservation != nil && (eta.Before(stay.Reservation.ArrivalDate.Add(-24*time.Hour)) || eta.After(stay.Reservation.DepartureDate)) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "زمان ورود خارج از بازه رزرو است")
		return
	}
	if len(input.Answers) == 0 {
		input.Answers = json.RawMessage(`{}`)
	}
	var answerObject map[string]any
	if err := json.Unmarshal(input.Answers, &answerObject); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "پاسخ‌های هتل معتبر نیست")
		return
	}
	journey, err := s.arrival.SaveArrivalDetails(r.Context(), stay.HotelID, stay.ID, store.ArrivalDetailsInput{
		ContactPhone: input.ContactPhone, ContactEmail: input.ContactEmail, Nationality: input.Nationality,
		ArrivalETA: eta, ArrivalMethod: input.ArrivalMethod, TransportDetails: strings.TrimSpace(input.TransportDetails),
		AccessibilityNeeds: strings.TrimSpace(input.AccessibilityNeeds), SpecialRequests: strings.TrimSpace(input.SpecialRequests), AnswersJSON: input.Answers,
	}, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

type companionInput struct {
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Relationship     string `json:"relationship"`
	Nationality      string `json:"nationality"`
	DateOfBirth      string `json:"dateOfBirth"`
	DocumentRequired bool   `json:"documentRequired"`
}

func (s *Server) replaceGuestArrivalCompanions(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	var input struct {
		Companions []companionInput `json:"companions"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Companions) > 8 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "حداکثر هشت همراه قابل ثبت است")
		return
	}
	companions := make([]models.ArrivalCompanion, 0, len(input.Companions))
	for _, item := range input.Companions {
		item.FirstName, item.LastName = strings.TrimSpace(item.FirstName), strings.TrimSpace(item.LastName)
		if len(item.FirstName) < 2 || len(item.LastName) < 2 || len(item.Relationship) > 64 || len(item.Nationality) > 80 {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "اطلاعات همراه معتبر نیست")
			return
		}
		companion := models.ArrivalCompanion{FirstName: item.FirstName, LastName: item.LastName, Relationship: strings.TrimSpace(item.Relationship), Nationality: strings.TrimSpace(item.Nationality), DocumentRequired: item.DocumentRequired}
		if item.DateOfBirth != "" {
			birth, err := time.Parse("2006-01-02", item.DateOfBirth)
			if err != nil || birth.After(time.Now().UTC()) {
				writeError(w, http.StatusUnprocessableEntity, "validation_failed", "تاریخ تولد همراه معتبر نیست")
				return
			}
			companion.DateOfBirth = &birth
		}
		companions = append(companions, companion)
	}
	journey, err := s.arrival.ReplaceArrivalCompanions(r.Context(), stay.HotelID, stay.ID, companions, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) uploadGuestArrivalDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	if s.documents == nil {
		writeError(w, http.StatusServiceUnavailable, "document_storage_unavailable", "ذخیره مدرک در دسترس نیست")
		return
	}
	stay, _ := currentStay(r)
	maxBytes := s.documentMaxBytes
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1024*1024)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "document_too_large", "حجم مدرک بیش از حد مجاز است")
		return
	}
	file, header, err := r.FormFile("document")
	if err != nil {
		writeError(w, http.StatusBadRequest, "document_required", "فایل مدرک الزامی است")
		return
	}
	defer file.Close()
	evidenceType := strings.TrimSpace(r.FormValue("evidenceType"))
	side := strings.TrimSpace(r.FormValue("side"))
	if evidenceType == "" {
		evidenceType = "identity"
	}
	if side == "" {
		side = "single"
	}
	if !map[string]bool{"identity": true, "passport": true, "visa": true, "other": true}[evidenceType] || !map[string]bool{"single": true, "front": true, "back": true}[side] {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "نوع مدرک معتبر نیست")
		return
	}
	var companionID *uuid.UUID
	if value := strings.TrimSpace(r.FormValue("companionId")); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "همراه معتبر نیست")
			return
		}
		companionID = &parsed
	}
	saved, err := s.documents.Save(r.Context(), stay.HotelID, file, header.Filename)
	if err != nil {
		writeDocumentSaveError(w, err)
		return
	}
	verificationState, verificationNote := "manual_review", ""
	opened, openErr := s.documents.Open(r.Context(), saved.StorageKey)
	if openErr == nil {
		result, verifyErr := s.identityVerifier.Verify(r.Context(), opened, saved.MediaType)
		_ = opened.Close()
		if verifyErr == nil && result.State != "" {
			verificationState, verificationNote = result.State, result.Note
		}
	}
	now := time.Now().UTC()
	retentionBase := now
	if stay.Reservation != nil && stay.Reservation.DepartureDate.After(retentionBase) {
		retentionBase = stay.Reservation.DepartureDate
	}
	retention := s.documentRetention
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	document, err := s.arrival.AddArrivalDocument(r.Context(), stay.HotelID, stay.ID, models.ArrivalDocument{
		CompanionID: companionID, EvidenceType: evidenceType, Side: side, StorageKey: saved.StorageKey, Name: saved.Name,
		MediaType: saved.MediaType, Size: saved.Size, SHA256: saved.SHA256, VerificationState: verificationState,
		VerificationNote: verificationNote, RetentionUntil: retentionBase.Add(retention),
	}, now)
	if err != nil {
		_ = s.documents.Delete(r.Context(), saved.StorageKey)
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.document.upload", models.AuditOutcomeSuccess, map[string]any{"journeyId": document.JourneyID, "documentId": document.ID, "mediaType": document.MediaType, "size": document.Size})
	writeJSON(w, http.StatusCreated, map[string]any{"document": toArrivalDocumentView(document)})
}

func (s *Server) deleteGuestArrivalDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_document", "شناسه مدرک معتبر نیست")
		return
	}
	key, err := s.arrival.DeleteArrivalDocument(r.Context(), stay.HotelID, stay.ID, id, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	if s.documents != nil {
		_ = s.documents.Delete(r.Context(), key)
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.document.delete", models.AuditOutcomeSuccess, map[string]any{"documentId": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) saveGuestArrivalSignature(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	if s.documents == nil {
		writeError(w, http.StatusServiceUnavailable, "document_storage_unavailable", "ذخیره امضا در دسترس نیست")
		return
	}
	stay, _ := currentStay(r)
	maxBytes := s.documentMaxBytes
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1024*1024)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "signature_too_large", "حجم امضا بیش از حد مجاز است")
		return
	}
	file, header, err := r.FormFile("signature")
	if err != nil {
		writeError(w, http.StatusBadRequest, "signature_required", "امضای دیجیتال الزامی است")
		return
	}
	defer file.Close()
	signerName := strings.TrimSpace(r.FormValue("signerName"))
	termsVersion := strings.TrimSpace(r.FormValue("termsVersion"))
	locale := strings.TrimSpace(r.FormValue("locale"))
	consent, _ := strconv.ParseBool(r.FormValue("consent"))
	if len(signerName) < 3 || len(signerName) > 180 || !consent || termsVersion == "" || locale == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "نام امضاکننده و رضایت صریح الزامی است")
		return
	}
	saved, err := s.documents.Save(r.Context(), stay.HotelID, file, header.Filename)
	if err != nil {
		writeDocumentSaveError(w, err)
		return
	}
	if saved.MediaType != "image/png" && saved.MediaType != "image/jpeg" {
		_ = s.documents.Delete(r.Context(), saved.StorageKey)
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_signature", "امضا باید تصویر PNG یا JPG باشد")
		return
	}
	journey, oldKey, err := s.arrival.SaveArrivalSignature(r.Context(), stay.HotelID, stay.ID, models.ArrivalJourney{
		TermsVersion: termsVersion, TermsLocale: locale, SignerName: signerName,
		SignatureStorageKey: saved.StorageKey, SignatureMediaType: saved.MediaType, SignatureSize: saved.Size, SignatureSHA256: saved.SHA256,
	}, time.Now().UTC())
	if err != nil {
		_ = s.documents.Delete(r.Context(), saved.StorageKey)
		writeArrivalError(w, err)
		return
	}
	if oldKey != "" && oldKey != saved.StorageKey {
		_ = s.documents.Delete(r.Context(), oldKey)
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.signature.save", models.AuditOutcomeSuccess, map[string]any{"journeyId": journey.ID, "termsVersion": termsVersion, "locale": locale})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) submitGuestArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		var input struct {
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		idempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	}
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "کلید تکرارناپذیری معتبر نیست")
		return
	}
	journey, err := s.arrival.SubmitArrival(r.Context(), stay.HotelID, stay.ID, idempotencyKey, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.submit", models.AuditOutcomeSuccess, map[string]any{"journeyId": journey.ID})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) cancelGuestArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	journey, err := s.arrival.CancelArrival(r.Context(), stay.HotelID, stay.ID, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "arrival.cancel", models.AuditOutcomeSuccess, map[string]any{"journeyId": journey.ID})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) recordGuestArrivalEvent(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	stay, _ := currentStay(r)
	journey, _, err := s.arrival.GetGuestArrival(r.Context(), stay.HotelID, stay.ID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	var input struct {
		Type       string `json:"type"`
		Step       int    `json:"step"`
		DurationMS int64  `json:"durationMs"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !map[string]bool{"start": true, "error": true, "abandon": true}[input.Type] || input.Step < 0 || input.Step > 3 || input.DurationMS < 0 || input.DurationMS > 24*60*60*1000 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "رویداد معتبر نیست")
		return
	}
	if err := s.arrival.RecordArrivalEvent(r.Context(), stay.HotelID, journey.ID, input.Type, input.Step, "guest", &stay.GuestID, input.DurationMS, time.Now().UTC()); err != nil {
		writeArrivalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getArrivalSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	settings, err := s.arrival.GetArrivalSettings(r.Context(), staff.HotelID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": toArrivalSettingsView(settings)})
}

func (s *Server) updateArrivalSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input struct {
		OnlineCheckInEnabled       bool            `json:"onlineCheckInEnabled"`
		DigitalRegistrationEnabled bool            `json:"digitalRegistrationEnabled"`
		PaymentStepEnabled         bool            `json:"paymentStepEnabled"`
		InvitationTTLHours         int             `json:"invitationTtlHours"`
		TermsVersion               string          `json:"termsVersion"`
		TermsLocale                string          `json:"termsLocale"`
		TermsText                  string          `json:"termsText"`
		Steps                      json.RawMessage `json:"steps"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.TermsVersion, input.TermsLocale, input.TermsText = strings.TrimSpace(input.TermsVersion), strings.TrimSpace(input.TermsLocale), strings.TrimSpace(input.TermsText)
	var steps []map[string]any
	if input.InvitationTTLHours < 1 || input.InvitationTTLHours > 720 || len(input.TermsVersion) < 1 || len(input.TermsVersion) > 64 || len(input.TermsLocale) < 2 || len(input.TermsLocale) > 12 || len(input.TermsText) < 20 || len(input.TermsText) > 10_000 || len(input.Steps) > 16*1024 || json.Unmarshal(input.Steps, &steps) != nil || len(steps) != 3 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "تنظیمات چک‌این معتبر نیست")
		return
	}
	settings, err := s.arrival.UpdateArrivalSettings(r.Context(), staff.HotelID, models.ArrivalSettings{
		OnlineCheckInEnabled: input.OnlineCheckInEnabled, DigitalRegistrationEnabled: input.DigitalRegistrationEnabled, PaymentStepEnabled: input.PaymentStepEnabled,
		InvitationTTLHours: input.InvitationTTLHours, TermsVersion: input.TermsVersion, TermsLocale: input.TermsLocale, TermsText: input.TermsText, StepsJSON: input.Steps,
	})
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.settings.update", models.AuditOutcomeSuccess, map[string]any{"enabled": settings.OnlineCheckInEnabled, "digitalRegistration": settings.DigitalRegistrationEnabled, "termsVersion": settings.TermsVersion})
	writeJSON(w, http.StatusOK, map[string]any{"settings": toArrivalSettingsView(settings)})
}

func (s *Server) listStaffArrivals(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	filter := store.ArrivalFilter{Status: models.ArrivalStatus(r.URL.Query().Get("status")), RiskState: strings.TrimSpace(r.URL.Query().Get("risk"))}
	if filter.Status != "" && !validArrivalStatus(filter.Status) {
		writeError(w, http.StatusBadRequest, "invalid_filter", "فیلتر وضعیت معتبر نیست")
		return
	}
	if r.URL.Query().Get("unassigned") == "true" {
		filter.Unassigned = true
	}
	if value := r.URL.Query().Get("ownerId"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "مالک بررسی معتبر نیست")
			return
		}
		filter.OwnerID = &parsed
	}
	journeys, err := s.arrival.ListArrivals(r.Context(), staff.HotelID, filter)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	views := make([]arrivalView, 0, len(journeys))
	for _, journey := range journeys {
		views = append(views, toArrivalView(journey))
	}
	writeJSON(w, http.StatusOK, map[string]any{"arrivals": views})
}

func (s *Server) assignArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	var input struct {
		OwnerID uuid.UUID `json:"ownerId"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.OwnerID == uuid.Nil {
		input.OwnerID = staff.ID
	}
	journey, err := s.arrival.AssignArrival(r.Context(), staff.HotelID, journeyID, staff.ID, input.OwnerID, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.assign", models.AuditOutcomeSuccess, map[string]any{"journeyId": journeyID, "ownerId": input.OwnerID})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) reviewArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	var input struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	status := models.ArrivalApproved
	if input.Decision == "needs_changes" {
		status = models.ArrivalNeedsChanges
	} else if input.Decision != "approve" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "نتیجه بررسی معتبر نیست")
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if status == models.ArrivalNeedsChanges && input.Reason == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "دلیل اصلاح الزامی است")
		return
	}
	journey, err := s.arrival.ReviewArrival(r.Context(), staff.HotelID, journeyID, staff.ID, status, input.Reason, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.review", models.AuditOutcomeSuccess, map[string]any{"journeyId": journeyID, "status": status})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) transitionArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	var input struct {
		Status models.ArrivalStatus `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status != models.ArrivalPending && input.Status != models.ArrivalRoomReady {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "وضعیت ورود معتبر نیست")
		return
	}
	journey, err := s.arrival.TransitionArrival(r.Context(), staff.HotelID, journeyID, staff.ID, input.Status, time.Now().UTC())
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.status.update", models.AuditOutcomeSuccess, map[string]any{"journeyId": journeyID, "status": input.Status})
	writeJSON(w, http.StatusOK, map[string]any{"arrival": toArrivalView(journey)})
}

func (s *Server) remindArrival(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	if err := s.arrival.RecordArrivalEvent(r.Context(), staff.HotelID, journeyID, "reminder", 0, "staff", &staff.ID, 0, time.Now().UTC()); err != nil {
		writeArrivalError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.reminder.queue", models.AuditOutcomeSuccess, map[string]any{"journeyId": journeyID, "delivery": "deferred_to_m10"})
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "delivery": "deferred_to_m10"})
}

func (s *Server) bulkRemindArrivals(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "بین یک تا صد ورود انتخاب کنید")
		return
	}
	now := time.Now().UTC()
	queued := 0
	for _, id := range input.IDs {
		if id != uuid.Nil && s.arrival.RecordArrivalEvent(r.Context(), staff.HotelID, id, "reminder", 0, "staff", &staff.ID, 0, now) == nil {
			queued++
		}
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "arrival.reminder.bulk.queue", models.AuditOutcomeSuccess, map[string]any{"requested": len(input.IDs), "queued": queued, "delivery": "deferred_to_m10"})
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": queued, "delivery": "deferred_to_m10"})
}

func (s *Server) arrivalAnalytics(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) {
		return
	}
	staff, _ := currentStaff(r)
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	if value := r.URL.Query().Get("from"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "زمان شروع معتبر نیست")
			return
		}
		from = parsed.UTC()
	}
	if value := r.URL.Query().Get("to"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filter", "زمان پایان معتبر نیست")
			return
		}
		to = parsed.UTC()
	}
	if !to.After(from) || to.Sub(from) > 366*24*time.Hour {
		writeError(w, http.StatusBadRequest, "invalid_filter", "بازه گزارش معتبر نیست")
		return
	}
	analytics, err := s.arrival.ArrivalAnalytics(r.Context(), staff.HotelID, from, to)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analytics": analytics})
}

func (s *Server) downloadArrivalDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) || s.documents == nil {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	documentID, err := uuid.Parse(r.PathValue("documentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_document", "شناسه مدرک معتبر نیست")
		return
	}
	document, err := s.arrival.GetArrivalDocument(r.Context(), staff.HotelID, journeyID, documentID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.servePrivateEvidence(w, r, document.StorageKey, document.Name, document.MediaType, document.CreatedAt)
}

func (s *Server) downloadArrivalSignature(w http.ResponseWriter, r *http.Request) {
	if !s.requireArrival(w) || s.documents == nil {
		return
	}
	staff, _ := currentStaff(r)
	journeyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_arrival", "شناسه ورود معتبر نیست")
		return
	}
	journey, err := s.arrival.GetArrivalSignature(r.Context(), staff.HotelID, journeyID)
	if err != nil {
		writeArrivalError(w, err)
		return
	}
	s.servePrivateEvidence(w, r, journey.SignatureStorageKey, "registration-signature.png", journey.SignatureMediaType, journey.UpdatedAt)
}

func (s *Server) servePrivateEvidence(w http.ResponseWriter, r *http.Request, key, name, mediaType string, modified time.Time) {
	file, err := s.documents.Open(r.Context(), key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusGone, "evidence_unavailable", "مدرک دیگر موجود نیست")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت مدرک انجام نشد")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", strings.ReplaceAll(name, `"`, "")))
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, modified, file)
}

func randomRecoveryCode() (string, error) {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buffer), nil
}

func checkInBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func writeDocumentSaveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, documents.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "document_too_large", "حجم فایل بیش از حد مجاز است")
	case errors.Is(err, documents.ErrUnsupportedType):
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_document", "فقط PDF، JPG یا PNG پذیرفته می‌شود")
	case errors.Is(err, documents.ErrUnsafeContent):
		writeError(w, http.StatusUnprocessableEntity, "unsafe_document", "فایل دارای محتوای فعال یا ناامن است")
	default:
		writeError(w, http.StatusInternalServerError, "internal_server_error", "ذخیره فایل انجام نشد")
	}
}

func writeArrivalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "not_found", "مورد درخواستی پیدا نشد")
	case errors.Is(err, store.ErrArrivalDisabled):
		writeError(w, http.StatusForbidden, "online_check_in_disabled", "چک‌این آنلاین برای این هتل فعال نیست")
	case errors.Is(err, store.ErrInvitationExpired):
		writeError(w, http.StatusGone, "invitation_expired", "دعوت منقضی شده است؛ کد بازیابی یا دعوت جدید دریافت کنید")
	case errors.Is(err, store.ErrInvitationRevoked):
		writeError(w, http.StatusGone, "invitation_revoked", "این دعوت لغو شده است")
	case errors.Is(err, store.ErrArrivalIncomplete):
		writeError(w, http.StatusUnprocessableEntity, "arrival_incomplete", "همه مراحل الزامی را تکمیل کنید")
	case errors.Is(err, store.ErrTermsOutdated):
		writeError(w, http.StatusConflict, "terms_changed", "مقررات هتل تغییر کرده است؛ متن جدید را مطالعه و دوباره امضا کنید")
	case errors.Is(err, store.ErrEvidenceUnavailable):
		writeError(w, http.StatusGone, "evidence_unavailable", "دوره نگهداری مدرک پایان یافته است")
	case errors.Is(err, store.ErrRoomUnavailable):
		writeError(w, http.StatusConflict, "room_unavailable", "اتاق هنوز آماده نیست")
	case errors.Is(err, store.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_transition", "این عملیات در وضعیت فعلی مجاز نیست")
	default:
		writeError(w, http.StatusInternalServerError, "internal_server_error", "عملیات آمادگی ورود انجام نشد")
	}
}

func validArrivalStatus(status models.ArrivalStatus) bool {
	if status == "" {
		return true
	}
	return map[models.ArrivalStatus]bool{
		models.ArrivalDraft: true, models.ArrivalSubmitted: true, models.ArrivalNeedsChanges: true, models.ArrivalApproved: true,
		models.ArrivalPending: true, models.ArrivalRoomReady: true, models.ArrivalCheckedIn: true, models.ArrivalExpired: true, models.ArrivalCancelled: true,
	}[status]
}
