package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const realtimeProtocol = "hotelmate.events"

func (s *Server) registerRealtimeRoute(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/events", s.realtimeEvents)
}

func (s *Server) realtimeEvents(w http.ResponseWriter, r *http.Request) {
	if s.realtime == nil || s.tokens == nil || s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "realtime_unavailable", "به‌روزرسانی زنده در دسترس نیست")
		return
	}
	if !s.realtimeOriginAllowed(r.Header.Get("Origin")) {
		writeError(w, http.StatusForbidden, "origin_forbidden", "مبدأ اتصال مجاز نیست")
		return
	}
	token := realtimeToken(r.Header.Get("Sec-WebSocket-Protocol"))
	principal, err := s.authorizeRealtime(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "نشست به‌روزرسانی زنده معتبر نیست")
		return
	}
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{realtimeProtocol}, InsecureSkipVerify: true,
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		s.logger.Warn("accept realtime connection", "error", err)
		return
	}
	defer connection.Close(websocket.StatusNormalClosure, "session ended")
	connection.SetReadLimit(1024)
	connectionContext := connection.CloseRead(context.Background())
	events, unsubscribe := s.realtime.Subscribe(principal)
	defer unsubscribe()
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	writeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = wsjson.Write(writeContext, connection, map[string]any{
		"type": "connected", "payload": map[string]any{"actorType": principal.ActorType}, "emittedAt": time.Now().UTC(),
	})
	cancel()
	if err != nil {
		return
	}
	for {
		select {
		case <-connectionContext.Done():
			return
		case <-heartbeat.C:
			pingContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := connection.Ping(pingContext)
			cancel()
			if err != nil {
				return
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			writeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := wsjson.Write(writeContext, connection, event)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) authorizeRealtime(ctx context.Context, token string) (realtime.Subscriber, error) {
	if claims, err := s.tokens.Parse(token, auth.ActorStaff); err == nil {
		hotelID, _ := claims.TenantID()
		staffID, _ := claims.SubjectID()
		staff, err := s.store.FindStaffByID(ctx, hotelID, staffID)
		if err != nil || !staff.IsActive {
			return realtime.Subscriber{}, errInvalidRealtimeSession
		}
		return realtime.Subscriber{ActorType: auth.ActorStaff, HotelID: hotelID, Role: staff.Role}, nil
	}
	claims, err := s.tokens.Parse(token, auth.ActorGuest)
	if err != nil {
		return realtime.Subscriber{}, errInvalidRealtimeSession
	}
	hotelID, _ := claims.TenantID()
	guestID, _ := claims.SubjectID()
	stayID, _ := claims.ActiveStayID()
	if _, err := s.store.FindGuestSession(ctx, hotelID, guestID, stayID); err != nil {
		return realtime.Subscriber{}, errInvalidRealtimeSession
	}
	return realtime.Subscriber{ActorType: auth.ActorGuest, HotelID: hotelID, StayID: stayID}, nil
}

func realtimeToken(header string) string {
	for _, item := range strings.Split(header, ",") {
		candidate := strings.TrimSpace(item)
		if candidate != "" && candidate != realtimeProtocol {
			return candidate
		}
	}
	return ""
}

func (s *Server) realtimeOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	for _, allowed := range s.allowedOrigins {
		if allowed == "*" || strings.EqualFold(strings.TrimRight(allowed, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

type realtimeSessionError struct{}

func (realtimeSessionError) Error() string { return "invalid realtime session" }

var errInvalidRealtimeSession realtimeSessionError
