package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) guestOnlineCheckIn(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	stay, _ := currentStay(r)
	if s.arrival != nil {
		settings, err := s.arrival.GetArrivalSettings(r.Context(), stay.HotelID)
		if err != nil || !settings.OnlineCheckInEnabled || !settings.DigitalRegistrationEnabled {
			writeError(w, http.StatusForbidden, "online_check_in_disabled", "چک‌این آنلاین برای این هتل فعال نیست")
			return
		}
	}
	checkIn, err := s.lifecycle.GetOnlineCheckInByStay(r.Context(), stay.HotelID, stay.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"onlineCheckIn": nil})
		return
	}
	if err != nil {
		s.logger.Error("get online check-in", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت چک‌این آنلاین انجام نشد")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"onlineCheckIn": toOnlineCheckInView(checkIn, false)})
}

func (s *Server) submitOnlineCheckIn(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	if s.documents == nil {
		writeError(w, http.StatusServiceUnavailable, "document_storage_unavailable", "ذخیره مدرک در دسترس نیست")
		return
	}
	stay, _ := currentStay(r)
	if s.arrival != nil {
		settings, err := s.arrival.GetArrivalSettings(r.Context(), stay.HotelID)
		if err != nil || !settings.OnlineCheckInEnabled || !settings.DigitalRegistrationEnabled {
			writeError(w, http.StatusForbidden, "online_check_in_disabled", "چک‌این آنلاین برای این هتل فعال نیست")
			return
		}
	}
	if stay.Status != models.StayPreArrival {
		writeError(w, http.StatusConflict, "invalid_transition", "چک‌این آنلاین فقط پیش از ورود امکان‌پذیر است")
		return
	}
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
	saved, err := s.documents.Save(r.Context(), stay.HotelID, file, header.Filename)
	if err != nil {
		writeDocumentSaveError(w, err)
		return
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
	document := models.OnlineCheckIn{
		DocumentStorageKey: saved.StorageKey, DocumentName: saved.Name, DocumentMediaType: saved.MediaType,
		DocumentSize: saved.Size, DocumentSHA256: saved.SHA256, SubmittedAt: now, RetentionUntil: retentionBase.Add(retention),
	}
	checkIn, oldKey, err := s.lifecycle.UpsertOnlineCheckIn(r.Context(), stay.HotelID, stay.ID, document)
	if err != nil {
		_ = s.documents.Delete(r.Context(), saved.StorageKey)
		writeLifecycleError(w, err)
		return
	}
	if oldKey != "" && oldKey != saved.StorageKey {
		if err := s.documents.Delete(r.Context(), oldKey); err != nil {
			s.logger.Error("delete replaced check-in document", "error", err)
		}
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "online_check_in.submit", models.AuditOutcomeSuccess, map[string]any{"stayId": stay.ID, "checkInId": checkIn.ID, "mediaType": saved.MediaType, "size": saved.Size})
	writeJSON(w, http.StatusCreated, map[string]any{"onlineCheckIn": toOnlineCheckInView(checkIn, false)})
}

func (s *Server) listOnlineCheckIns(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	status := models.OnlineCheckInStatus(r.URL.Query().Get("status"))
	if !validOnlineCheckInStatus(status) {
		writeError(w, http.StatusBadRequest, "invalid_filter", "فیلتر وضعیت معتبر نیست")
		return
	}
	checkIns, err := s.lifecycle.ListOnlineCheckIns(r.Context(), staff.HotelID, status)
	if err != nil {
		s.logger.Error("list online check-ins", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت چک‌این‌ها انجام نشد")
		return
	}
	views := make([]onlineCheckInView, 0, len(checkIns))
	for _, checkIn := range checkIns {
		views = append(views, toOnlineCheckInView(checkIn, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"onlineCheckIns": views})
}

type reviewOnlineCheckInRequest struct {
	Status models.OnlineCheckInStatus `json:"status"`
	Note   string                     `json:"note"`
}

func (s *Server) reviewOnlineCheckIn(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	staff, _ := currentStaff(r)
	checkInID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_check_in", "شناسه چک‌این معتبر نیست")
		return
	}
	var input reviewOnlineCheckInRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if (input.Status != models.OnlineCheckInApproved && input.Status != models.OnlineCheckInRejected) || len(input.Note) > 500 || (input.Status == models.OnlineCheckInRejected && input.Note == "") {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "نتیجه بررسی معتبر نیست")
		return
	}
	checkIn, err := s.lifecycle.ReviewOnlineCheckIn(r.Context(), staff.HotelID, checkInID, staff.ID, input.Status, input.Note, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "online_check_in.review", models.AuditOutcomeSuccess, map[string]any{"checkInId": checkIn.ID, "status": checkIn.Status})
	writeJSON(w, http.StatusOK, map[string]any{"onlineCheckIn": toOnlineCheckInView(checkIn, false)})
}

func (s *Server) downloadOnlineCheckInDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireLifecycle(w) {
		return
	}
	if s.documents == nil {
		writeError(w, http.StatusServiceUnavailable, "document_storage_unavailable", "مدرک در دسترس نیست")
		return
	}
	staff, _ := currentStaff(r)
	checkInID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_check_in", "شناسه چک‌این معتبر نیست")
		return
	}
	checkIn, err := s.lifecycle.GetOnlineCheckIn(r.Context(), staff.HotelID, checkInID)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	if checkIn.DocumentDeletedAt != nil || !time.Now().UTC().Before(checkIn.RetentionUntil) {
		writeError(w, http.StatusGone, "document_expired", "دوره نگهداری مدرک پایان یافته است")
		return
	}
	file, err := s.documents.Open(r.Context(), checkIn.DocumentStorageKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusGone, "document_unavailable", "مدرک دیگر موجود نیست")
			return
		}
		s.logger.Error("open online check-in document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_server_error", "دریافت مدرک انجام نشد")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", checkIn.DocumentMediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", strings.ReplaceAll(checkIn.DocumentName, `"`, "")))
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, checkIn.DocumentName, checkIn.SubmittedAt, file)
}
