package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/google/uuid"
)

func (s *Server) audit(r *http.Request, hotelID, actorID *uuid.UUID, actorType, action string, outcome models.AuditOutcome, metadata map[string]any) {
	if s.store == nil {
		return
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		encoded = json.RawMessage(`{}`)
	}
	entry := &models.AuditLog{
		HotelID: hotelID, ActorID: actorID, ActorType: actorType, Action: action,
		Outcome: outcome, IPAddress: clientIP(r), RequestID: requestID(r), Metadata: encoded,
	}
	if err := s.store.WriteAudit(r.Context(), entry); err != nil {
		s.logger.Error("write audit log", "requestId", requestID(r), "action", action, "error", err)
	}
}

func clientIP(r *http.Request) string {
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
