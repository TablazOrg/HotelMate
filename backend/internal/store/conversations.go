package store

import (
	"context"
	"errors"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrConversationClosed       = errors.New("conversation is closed")
	ErrConversationNotHandedOff = errors.New("conversation is not handed off")
	ErrKnowledgeNotPending      = errors.New("knowledge item is not pending review")
	ErrKnowledgeVersionPending  = errors.New("a pending version already exists")
)

type ConversationStore interface {
	GetOrCreateGuestConversation(context.Context, models.Stay, time.Time, time.Time) (models.Conversation, error)
	AddGuestMessage(context.Context, models.Stay, uuid.UUID, string, bool, time.Time, time.Time) (models.Conversation, error)
	AddAssistantReply(context.Context, uuid.UUID, uuid.UUID, string, *float64, *uuid.UUID, bool, time.Time, time.Time) (models.Conversation, error)
	ListStaffConversations(context.Context, uuid.UUID, models.ConversationStatus, time.Time) ([]models.Conversation, error)
	GetStaffConversation(context.Context, uuid.UUID, uuid.UUID, time.Time) (models.Conversation, error)
	AddStaffMessage(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, bool, time.Time, time.Time) (models.Conversation, error)
	MarkGuestConversationRead(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time, time.Time) (models.Conversation, error)
	MarkStaffConversationRead(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time) (models.Conversation, error)
	CloseConversation(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time, time.Time) (models.Conversation, error)
	ListApprovedKnowledge(context.Context, uuid.UUID) ([]models.KnowledgeItem, error)
	ListKnowledge(context.Context, uuid.UUID) ([]models.KnowledgeItem, error)
	CreateKnowledgeVersion(context.Context, uuid.UUID, uuid.UUID, string, string, string, *uuid.UUID) (models.KnowledgeItem, error)
	ReviewKnowledge(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.KnowledgeStatus, string, time.Time) (models.KnowledgeItem, error)
	PurgeExpiredMessages(context.Context, time.Time) (int64, error)
}

func (s *GORMStore) GetOrCreateGuestConversation(ctx context.Context, stay models.Stay, at, expiresAt time.Time) (models.Conversation, error) {
	var conversation models.Conversation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hotel_id = ? AND stay_id = ?", stay.HotelID, stay.ID).First(&conversation).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		stayID := stay.ID
		readAt := at
		conversation = models.Conversation{
			HotelID: stay.HotelID, GuestID: stay.GuestID, StayID: &stayID, Status: models.ConversationAI,
			GuestReadAt: &readAt, LastMessageAt: at,
		}
		if err := tx.Create(&conversation).Error; err != nil {
			return err
		}
		welcome := models.Message{
			BaseModel: models.BaseModel{CreatedAt: at}, ConversationID: conversation.ID, Role: models.MessageAI,
			Body: "سلام! من دستیار هوشمند هتل هستم. درباره امکانات، ساعت سرویس‌ها و اقامت از من بپرسید.", ExpiresAt: expiresAt,
		}
		return tx.Create(&welcome).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return s.loadGuestConversation(ctx, stay.HotelID, stay.ID, at)
		}
		return models.Conversation{}, err
	}
	return s.loadGuestConversation(ctx, stay.HotelID, stay.ID, at)
}

func (s *GORMStore) AddGuestMessage(ctx context.Context, stay models.Stay, conversationID uuid.UUID, body string, redacted bool, at, expiresAt time.Time) (models.Conversation, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hotel_id = ? AND stay_id = ? AND guest_id = ? AND id = ?", stay.HotelID, stay.ID, stay.GuestID, conversationID).
			First(&conversation).Error; err != nil {
			return err
		}
		if conversation.Status == models.ConversationClosed {
			return ErrConversationClosed
		}
		senderID := stay.GuestID
		message := models.Message{
			BaseModel: models.BaseModel{CreatedAt: at}, ConversationID: conversation.ID, Role: models.MessageGuest,
			SenderID: &senderID, Body: body, Redacted: redacted, ExpiresAt: expiresAt,
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		return tx.Model(&conversation).Updates(map[string]any{"last_message_at": at, "guest_read_at": at}).Error
	})
	if err != nil {
		return models.Conversation{}, err
	}
	return s.loadGuestConversation(ctx, stay.HotelID, stay.ID, at)
}

func (s *GORMStore) AddAssistantReply(ctx context.Context, hotelID, conversationID uuid.UUID, body string, confidence *float64, knowledgeID *uuid.UUID, handoff bool, at, expiresAt time.Time) (models.Conversation, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, conversationID).First(&conversation).Error; err != nil {
			return err
		}
		if conversation.Status == models.ConversationClosed {
			return ErrConversationClosed
		}
		message := models.Message{
			BaseModel: models.BaseModel{CreatedAt: at}, ConversationID: conversation.ID, Role: models.MessageAI,
			Body: body, Confidence: confidence, KnowledgeItemID: knowledgeID, ExpiresAt: expiresAt,
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		updates := map[string]any{"last_message_at": at}
		if handoff {
			updates["status"] = models.ConversationHandedOff
		}
		return tx.Model(&conversation).Updates(updates).Error
	})
	if err != nil {
		return models.Conversation{}, err
	}
	return s.loadStaffConversation(ctx, hotelID, conversationID, at)
}

func (s *GORMStore) ListStaffConversations(ctx context.Context, hotelID uuid.UUID, status models.ConversationStatus, at time.Time) ([]models.Conversation, error) {
	query := s.conversationPreloads(s.db.WithContext(ctx), at).Where("conversations.hotel_id = ?", hotelID)
	if status != "" {
		query = query.Where("conversations.status = ?", status)
	}
	var conversations []models.Conversation
	err := query.Order("conversations.last_message_at DESC").Find(&conversations).Error
	return conversations, err
}

func (s *GORMStore) GetStaffConversation(ctx context.Context, hotelID, conversationID uuid.UUID, at time.Time) (models.Conversation, error) {
	return s.loadStaffConversation(ctx, hotelID, conversationID, at)
}

func (s *GORMStore) AddStaffMessage(ctx context.Context, hotelID, conversationID, staffID uuid.UUID, body string, redacted bool, at, expiresAt time.Time) (models.Conversation, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, conversationID).First(&conversation).Error; err != nil {
			return err
		}
		if conversation.Status == models.ConversationClosed {
			return ErrConversationClosed
		}
		if conversation.Status != models.ConversationHandedOff {
			return ErrConversationNotHandedOff
		}
		message := models.Message{
			BaseModel: models.BaseModel{CreatedAt: at}, ConversationID: conversation.ID, Role: models.MessageStaff,
			SenderID: &staffID, Body: body, Redacted: redacted, ExpiresAt: expiresAt,
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		updates := map[string]any{"last_message_at": at, "staff_read_at": at}
		if conversation.AssignedToID == nil {
			updates["assigned_to_id"] = staffID
		}
		return tx.Model(&conversation).Updates(updates).Error
	})
	if err != nil {
		return models.Conversation{}, err
	}
	return s.loadStaffConversation(ctx, hotelID, conversationID, at)
}

func (s *GORMStore) MarkGuestConversationRead(ctx context.Context, hotelID, stayID, conversationID uuid.UUID, at, visibleAt time.Time) (models.Conversation, error) {
	result := s.db.WithContext(ctx).Model(&models.Conversation{}).
		Where("hotel_id = ? AND stay_id = ? AND id = ?", hotelID, stayID, conversationID).Update("guest_read_at", at)
	if result.Error != nil {
		return models.Conversation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.Conversation{}, gorm.ErrRecordNotFound
	}
	return s.loadGuestConversation(ctx, hotelID, stayID, visibleAt)
}

func (s *GORMStore) MarkStaffConversationRead(ctx context.Context, hotelID, conversationID uuid.UUID, at, visibleAt time.Time) (models.Conversation, error) {
	result := s.db.WithContext(ctx).Model(&models.Conversation{}).
		Where("hotel_id = ? AND id = ?", hotelID, conversationID).Update("staff_read_at", at)
	if result.Error != nil {
		return models.Conversation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.Conversation{}, gorm.ErrRecordNotFound
	}
	return s.loadStaffConversation(ctx, hotelID, conversationID, visibleAt)
}

func (s *GORMStore) CloseConversation(ctx context.Context, hotelID, conversationID, staffID uuid.UUID, at, expiresAt time.Time) (models.Conversation, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, conversationID).First(&conversation).Error; err != nil {
			return err
		}
		if conversation.Status == models.ConversationClosed {
			return ErrConversationClosed
		}
		message := models.Message{
			BaseModel: models.BaseModel{CreatedAt: at}, ConversationID: conversation.ID, Role: models.MessageSystem,
			SenderID: &staffID, Body: "گفتگو توسط پذیرش بسته شد.", ExpiresAt: expiresAt,
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		return tx.Model(&conversation).Updates(map[string]any{"status": models.ConversationClosed, "last_message_at": at, "staff_read_at": at}).Error
	})
	if err != nil {
		return models.Conversation{}, err
	}
	return s.loadStaffConversation(ctx, hotelID, conversationID, at)
}

func (s *GORMStore) ListApprovedKnowledge(ctx context.Context, hotelID uuid.UUID) ([]models.KnowledgeItem, error) {
	var items []models.KnowledgeItem
	err := s.db.WithContext(ctx).
		Where("knowledge_items.hotel_id = ? AND knowledge_items.status = ?", hotelID, models.KnowledgeApproved).
		Where("NOT EXISTS (?)", s.db.Model(&models.KnowledgeItem{}).Select("1").
			Where("newer.supersedes_id = knowledge_items.id AND newer.status = ? AND newer.deleted_at IS NULL", models.KnowledgeApproved).Table("knowledge_items AS newer")).
		Order("knowledge_items.title ASC").Find(&items).Error
	return items, err
}

func (s *GORMStore) ListKnowledge(ctx context.Context, hotelID uuid.UUID) ([]models.KnowledgeItem, error) {
	var items []models.KnowledgeItem
	err := s.db.WithContext(ctx).Where("hotel_id = ?", hotelID).
		Order("CASE status WHEN 'pending_review' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END, updated_at DESC").Find(&items).Error
	return items, err
}

func (s *GORMStore) CreateKnowledgeVersion(ctx context.Context, hotelID, staffID uuid.UUID, title, content, source string, supersedesID *uuid.UUID) (models.KnowledgeItem, error) {
	item := models.KnowledgeItem{
		HotelID: hotelID, Title: title, Content: content, Source: source, Status: models.KnowledgePending,
		Version: 1, SupersedesID: supersedesID, SubmittedByID: &staffID,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if supersedesID != nil {
			var previous models.KnowledgeItem
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, *supersedesID).First(&previous).Error; err != nil {
				return err
			}
			var pending int64
			if err := tx.Model(&models.KnowledgeItem{}).Where("hotel_id = ? AND supersedes_id = ? AND status = ?", hotelID, *supersedesID, models.KnowledgePending).Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return ErrKnowledgeVersionPending
			}
			item.Version = previous.Version + 1
		}
		return tx.Create(&item).Error
	})
	return item, err
}

func (s *GORMStore) ReviewKnowledge(ctx context.Context, hotelID, itemID, staffID uuid.UUID, status models.KnowledgeStatus, note string, at time.Time) (models.KnowledgeItem, error) {
	var item models.KnowledgeItem
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("hotel_id = ? AND id = ?", hotelID, itemID).First(&item).Error; err != nil {
			return err
		}
		if item.Status != models.KnowledgePending {
			return ErrKnowledgeNotPending
		}
		return tx.Model(&item).Updates(map[string]any{
			"status": status, "reviewed_by_id": staffID, "reviewed_at": at, "review_note": note,
		}).Error
	})
	if err != nil {
		return models.KnowledgeItem{}, err
	}
	err = s.db.WithContext(ctx).Where("hotel_id = ? AND id = ?", hotelID, itemID).First(&item).Error
	return item, err
}

func (s *GORMStore) PurgeExpiredMessages(ctx context.Context, before time.Time) (int64, error) {
	// Retention is a privacy boundary, so expired message bodies are physically
	// deleted instead of using GORM's recoverable soft-delete path.
	result := s.db.WithContext(ctx).Unscoped().Where("expires_at <= ?", before).Delete(&models.Message{})
	return result.RowsAffected, result.Error
}

func (s *GORMStore) loadGuestConversation(ctx context.Context, hotelID, stayID uuid.UUID, at time.Time) (models.Conversation, error) {
	var conversation models.Conversation
	err := s.conversationPreloads(s.db.WithContext(ctx), at).
		Where("conversations.hotel_id = ? AND conversations.stay_id = ?", hotelID, stayID).First(&conversation).Error
	return conversation, err
}

func (s *GORMStore) loadStaffConversation(ctx context.Context, hotelID, conversationID uuid.UUID, at time.Time) (models.Conversation, error) {
	var conversation models.Conversation
	err := s.conversationPreloads(s.db.WithContext(ctx), at).
		Where("conversations.hotel_id = ? AND conversations.id = ?", hotelID, conversationID).First(&conversation).Error
	return conversation, err
}

func (s *GORMStore) conversationPreloads(query *gorm.DB, at time.Time) *gorm.DB {
	return query.Preload("Guest").Preload("Stay.Room").Preload("AssignedTo").
		Preload("Messages", func(messages *gorm.DB) *gorm.DB {
			return messages.Where("expires_at > ?", at).Order("created_at ASC")
		})
}

var _ ConversationStore = (*GORMStore)(nil)
