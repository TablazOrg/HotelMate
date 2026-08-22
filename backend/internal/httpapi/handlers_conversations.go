package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/concierge"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var conversationRoles = []models.StaffRole{
	models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin, models.StaffRoleOperations, models.StaffRoleReception,
}

var knowledgeReviewRoles = []models.StaffRole{models.StaffRolePrimaryAdmin, models.StaffRoleSecondaryAdmin}

const handoffReply = "پاسخ دقیق این سؤال را ندارم؛ گفتگو را به پذیرش منتقل می‌کنم."

func (s *Server) registerConversationRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/guest/conversation", s.require(auth.ActorGuest)(http.HandlerFunc(s.guestConversation)))
	mux.Handle("POST /api/v1/guest/conversation/messages", s.require(auth.ActorGuest)(http.HandlerFunc(s.createGuestMessage)))
	mux.Handle("POST /api/v1/guest/conversation/read", s.require(auth.ActorGuest)(http.HandlerFunc(s.markGuestConversationRead)))
	mux.Handle("GET /api/v1/staff/conversations", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.listStaffConversations)))
	mux.Handle("GET /api/v1/staff/conversations/{id}", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.getStaffConversation)))
	mux.Handle("POST /api/v1/staff/conversations/{id}/messages", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.createStaffMessage)))
	mux.Handle("POST /api/v1/staff/conversations/{id}/read", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.markStaffConversationRead)))
	mux.Handle("POST /api/v1/staff/conversations/{id}/close", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.closeStaffConversation)))
	mux.Handle("GET /api/v1/staff/knowledge", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.listKnowledge)))
	mux.Handle("POST /api/v1/staff/knowledge", s.require(auth.ActorStaff, conversationRoles...)(http.HandlerFunc(s.createKnowledge)))
	mux.Handle("POST /api/v1/staff/knowledge/{id}/review", s.require(auth.ActorStaff, knowledgeReviewRoles...)(http.HandlerFunc(s.reviewKnowledge)))
}

func (s *Server) requireConversations(w http.ResponseWriter) bool {
	if s.conversations == nil {
		writeError(w, http.StatusServiceUnavailable, "conversation_unavailable", "گفتگو در دسترس نیست")
		return false
	}
	return true
}

func (s *Server) guestConversation(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	stay, _ := currentStay(r)
	now := time.Now().UTC()
	conversation, err := s.conversations.GetOrCreateGuestConversation(r.Context(), stay, now, now.Add(s.chatRetention))
	if err == nil {
		conversation, err = s.conversations.MarkGuestConversationRead(r.Context(), stay.HotelID, stay.ID, conversation.ID, now, now)
	}
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conversation)})
}

type conversationMessageInput struct {
	Body string `json:"body"`
}

func (s *Server) createGuestMessage(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	stay, _ := currentStay(r)
	var input conversationMessageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	original := strings.TrimSpace(input.Body)
	if len([]rune(original)) < 1 || len([]rune(original)) > 1000 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "پیام باید بین ۱ تا ۱۰۰۰ نویسه باشد")
		return
	}
	injection := concierge.IsPromptInjection(original)
	body, redacted := concierge.SanitizeGuestText(original)
	if body == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "متن پیام معتبر نیست")
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.chatRetention)
	conversation, err := s.conversations.GetOrCreateGuestConversation(r.Context(), stay, now, expiresAt)
	if err == nil {
		conversation, err = s.conversations.AddGuestMessage(r.Context(), stay, conversation.ID, body, redacted, now.Add(time.Millisecond), expiresAt)
	}
	if err != nil {
		writeConversationError(w, err)
		return
	}
	handedOff := false
	if conversation.Status == models.ConversationAI {
		answer := concierge.Answer{}
		if !injection {
			answer, err = s.answerFromApprovedKnowledge(r, stay.HotelID, body)
			if err != nil {
				s.logger.Error("approved knowledge answer", "error", err, "conversationId", conversation.ID)
			}
		}
		handedOff = injection || err != nil || answer.Body == "" || answer.Confidence < s.chatConfidence
		if handedOff {
			confidence := 0.0
			if answer.Confidence > 0 {
				confidence = answer.Confidence
			}
			conversation, err = s.conversations.AddAssistantReply(r.Context(), stay.HotelID, conversation.ID, handoffReply, &confidence, nil, true, now.Add(2*time.Millisecond), expiresAt)
		} else {
			confidence := answer.Confidence
			conversation, err = s.conversations.AddAssistantReply(r.Context(), stay.HotelID, conversation.ID, answer.Body, &confidence, answer.KnowledgeID, false, now.Add(2*time.Millisecond), expiresAt)
		}
		if err != nil {
			writeConversationError(w, err)
			return
		}
	}
	s.audit(r, &stay.HotelID, &stay.GuestID, "guest", "conversation.message", models.AuditOutcomeSuccess, map[string]any{
		"conversationId": conversation.ID, "redacted": redacted, "handedOff": handedOff, "injectionDetected": injection,
	})
	eventType := "message.created"
	if handedOff {
		eventType = "conversation.handoff"
	}
	s.publishConversation(eventType, conversation)
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) answerFromApprovedKnowledge(r *http.Request, hotelID uuid.UUID, body string) (concierge.Answer, error) {
	items, err := s.conversations.ListApprovedKnowledge(r.Context(), hotelID)
	if err != nil {
		return concierge.Answer{}, err
	}
	approved := make([]concierge.Knowledge, 0, len(items))
	for _, item := range items {
		approved = append(approved, concierge.Knowledge{ID: item.ID, Question: item.Title, Answer: item.Content})
	}
	return s.concierge.Answer(r.Context(), body, approved)
}

func (s *Server) markGuestConversationRead(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	stay, _ := currentStay(r)
	now := time.Now().UTC()
	conversation, err := s.conversations.GetOrCreateGuestConversation(r.Context(), stay, now, now.Add(s.chatRetention))
	if err == nil {
		conversation, err = s.conversations.MarkGuestConversationRead(r.Context(), stay.HotelID, stay.ID, conversation.ID, now, now)
	}
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) listStaffConversations(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	status := models.ConversationStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != models.ConversationAI && status != models.ConversationHandedOff && status != models.ConversationClosed {
		writeError(w, http.StatusBadRequest, "invalid_status", "وضعیت گفتگو معتبر نیست")
		return
	}
	conversations, err := s.conversations.ListStaffConversations(r.Context(), staff.HotelID, status, time.Now().UTC())
	if err != nil {
		writeConversationError(w, err)
		return
	}
	views := make([]conversationView, 0, len(conversations))
	for _, conversation := range conversations {
		views = append(views, toConversationView(conversation))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": views})
}

func (s *Server) getStaffConversation(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseConversationID(w, r)
	if !ok {
		return
	}
	conversation, err := s.conversations.GetStaffConversation(r.Context(), staff.HotelID, id, time.Now().UTC())
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) createStaffMessage(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseConversationID(w, r)
	if !ok {
		return
	}
	var input conversationMessageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	body, redacted := concierge.SanitizeGuestText(input.Body)
	if len([]rune(body)) < 1 || len([]rune(body)) > 1000 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "پیام باید بین ۱ تا ۱۰۰۰ نویسه باشد")
		return
	}
	now := time.Now().UTC()
	conversation, err := s.conversations.AddStaffMessage(r.Context(), staff.HotelID, id, staff.ID, body, redacted, now, now.Add(s.chatRetention))
	if err != nil {
		writeConversationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "conversation.reply", models.AuditOutcomeSuccess, map[string]any{"conversationId": id, "redacted": redacted})
	s.publishConversation("message.created", conversation)
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) markStaffConversationRead(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseConversationID(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	conversation, err := s.conversations.MarkStaffConversationRead(r.Context(), staff.HotelID, id, now, now)
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) closeStaffConversation(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseConversationID(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	conversation, err := s.conversations.CloseConversation(r.Context(), staff.HotelID, id, staff.ID, now, now.Add(s.chatRetention))
	if err != nil {
		writeConversationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "conversation.close", models.AuditOutcomeSuccess, map[string]any{"conversationId": id})
	s.publishConversation("conversation.updated", conversation)
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conversation)})
}

func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	items, err := s.conversations.ListKnowledge(r.Context(), staff.HotelID)
	if err != nil {
		writeConversationError(w, err)
		return
	}
	if items == nil {
		items = []models.KnowledgeItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge": items})
}

type knowledgeInput struct {
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	Source       string     `json:"source"`
	SupersedesID *uuid.UUID `json:"supersedesId"`
}

func (s *Server) createKnowledge(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	var input knowledgeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		input.Source = "پیشنهاد پرسنل"
	}
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 240 || len([]rune(input.Content)) < 2 || len([]rune(input.Content)) > 2000 || len([]rune(input.Source)) > 120 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "محتوای دانش‌نامه معتبر نیست")
		return
	}
	item, err := s.conversations.CreateKnowledgeVersion(r.Context(), staff.HotelID, staff.ID, input.Title, input.Content, input.Source, input.SupersedesID)
	if err != nil {
		writeConversationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "knowledge.submit", models.AuditOutcomeSuccess, map[string]any{"knowledgeId": item.ID, "version": item.Version})
	writeJSON(w, http.StatusCreated, map[string]any{"knowledgeItem": item})
}

type knowledgeReviewInput struct {
	Status models.KnowledgeStatus `json:"status"`
	Note   string                 `json:"note"`
}

func (s *Server) reviewKnowledge(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	staff, _ := currentStaff(r)
	id, ok := parseConversationID(w, r)
	if !ok {
		return
	}
	var input knowledgeReviewInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if (input.Status != models.KnowledgeApproved && input.Status != models.KnowledgeRejected) || len([]rune(input.Note)) > 500 || (input.Status == models.KnowledgeRejected && input.Note == "") {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "نتیجه بررسی معتبر نیست")
		return
	}
	item, err := s.conversations.ReviewKnowledge(r.Context(), staff.HotelID, id, staff.ID, input.Status, input.Note, time.Now().UTC())
	if err != nil {
		writeConversationError(w, err)
		return
	}
	s.audit(r, &staff.HotelID, &staff.ID, "staff", "knowledge.review", models.AuditOutcomeSuccess, map[string]any{"knowledgeId": item.ID, "status": item.Status, "version": item.Version})
	writeJSON(w, http.StatusOK, map[string]any{"knowledgeItem": item})
}

func parseConversationID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "شناسه معتبر نیست")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) publishConversation(eventType string, conversation models.Conversation) {
	if s.realtime == nil || conversation.StayID == nil {
		return
	}
	s.realtime.Publish(realtime.Event{
		Type: eventType, Payload: map[string]any{"conversation": toConversationView(conversation)},
		HotelID: conversation.HotelID, StayID: *conversation.StayID, FulfillmentRole: models.StaffRoleReception,
	})
}

func writeConversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "not_found", "گفتگو یا محتوای درخواستی پیدا نشد")
	case errors.Is(err, store.ErrConversationClosed):
		writeError(w, http.StatusConflict, "conversation_closed", "این گفتگو بسته شده است")
	case errors.Is(err, store.ErrConversationNotHandedOff):
		writeError(w, http.StatusConflict, "conversation_not_handed_off", "این گفتگو هنوز به پذیرش منتقل نشده است")
	case errors.Is(err, store.ErrKnowledgeNotPending):
		writeError(w, http.StatusConflict, "knowledge_not_pending", "این نسخه قبلاً بررسی شده است")
	case errors.Is(err, store.ErrKnowledgeVersionPending):
		writeError(w, http.StatusConflict, "knowledge_version_pending", "یک نسخه جدید از این محتوا در انتظار بررسی است")
	default:
		writeError(w, http.StatusInternalServerError, "internal_server_error", "عملیات گفتگو انجام نشد")
	}
}

type messageView struct {
	ID         uuid.UUID          `json:"id"`
	Role       models.MessageRole `json:"role"`
	Body       string             `json:"body"`
	SenderName string             `json:"senderName"`
	Confidence *float64           `json:"confidence,omitempty"`
	Redacted   bool               `json:"redacted"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type conversationView struct {
	ID               uuid.UUID                 `json:"id"`
	Status           models.ConversationStatus `json:"status"`
	Guest            guestView                 `json:"guest"`
	Stay             *conversationStayView     `json:"stay,omitempty"`
	AssignedTo       *staffView                `json:"assignedTo"`
	GuestUnreadCount int                       `json:"guestUnreadCount"`
	StaffUnreadCount int                       `json:"staffUnreadCount"`
	LastMessageAt    time.Time                 `json:"lastMessageAt"`
	Messages         []messageView             `json:"messages"`
}

type conversationStayView struct {
	ID   uuid.UUID `json:"id"`
	Room roomView  `json:"room"`
}

func toConversationView(conversation models.Conversation) conversationView {
	view := conversationView{
		ID: conversation.ID, Status: conversation.Status,
		Guest:         guestView{ID: conversation.Guest.ID, FirstName: conversation.Guest.FirstName, LastName: conversation.Guest.LastName},
		LastMessageAt: conversation.LastMessageAt, Messages: make([]messageView, 0, len(conversation.Messages)),
	}
	if conversation.Stay != nil {
		view.Stay = &conversationStayView{ID: conversation.Stay.ID, Room: toRoomView(conversation.Stay.Room)}
	}
	staffName := "پذیرش هتل"
	if conversation.AssignedTo != nil {
		staff := toStaffView(*conversation.AssignedTo)
		view.AssignedTo = &staff
		staffName = conversation.AssignedTo.FirstName + " " + conversation.AssignedTo.LastName
	}
	for _, message := range conversation.Messages {
		senderName := "دستیار هوشمند"
		switch message.Role {
		case models.MessageGuest:
			senderName = "شما"
			if conversation.StaffReadAt == nil || message.CreatedAt.After(*conversation.StaffReadAt) {
				view.StaffUnreadCount++
			}
		case models.MessageStaff:
			senderName = staffName + " · پذیرش"
			if conversation.GuestReadAt == nil || message.CreatedAt.After(*conversation.GuestReadAt) {
				view.GuestUnreadCount++
			}
		case models.MessageAI, models.MessageSystem:
			if conversation.GuestReadAt == nil || message.CreatedAt.After(*conversation.GuestReadAt) {
				view.GuestUnreadCount++
			}
		}
		view.Messages = append(view.Messages, messageView{
			ID: message.ID, Role: message.Role, Body: message.Body, SenderName: senderName,
			Confidence: message.Confidence, Redacted: message.Redacted, CreatedAt: message.CreatedAt,
		})
	}
	return view
}
