package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/realtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

func TestRealtimeWebSocketAuthenticatesAndDeliversScopedEvent(t *testing.T) {
	const allowedOrigin = "https://hotel.example"
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Realtime Hotel", Slug: "realtime-hotel"}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleReception, IsActive: true}
	base := &fakeStore{hotel: hotel, staff: staff}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(staff)
	hub := realtime.NewHub()
	server := httptest.NewServer(NewHandler(Dependencies{Store: base, Tokens: tokens, Realtime: hub, AllowedOrigins: []string{allowedOrigin}}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/v1/events", &websocket.DialOptions{
		Subprotocols: []string{realtimeProtocol, token}, HTTPHeader: http.Header{"Origin": []string{allowedOrigin}},
	})
	if err != nil {
		t.Fatalf("dial realtime endpoint: %v", err)
	}
	defer connection.Close(websocket.StatusNormalClosure, "test complete")
	var connected struct {
		Type string `json:"type"`
	}
	if err := wsjson.Read(ctx, connection, &connected); err != nil || connected.Type != "connected" {
		t.Fatalf("read connected event: %+v err=%v", connected, err)
	}
	hub.Publish(realtime.Event{Type: "request.created", HotelID: hotel.ID, StayID: uuid.New(), Category: models.ServiceCategoryOther, FulfillmentRole: models.StaffRoleReception, Payload: map[string]string{"id": "request-1"}})
	var event struct {
		Type    string            `json:"type"`
		Payload map[string]string `json:"payload"`
	}
	if err := wsjson.Read(ctx, connection, &event); err != nil || event.Type != "request.created" || event.Payload["id"] != "request-1" {
		t.Fatalf("read operational event: %+v err=%v", event, err)
	}
}

func TestRealtimeWebSocketRejectsUntrustedOrigin(t *testing.T) {
	server := httptest.NewServer(NewHandler(Dependencies{Store: &fakeStore{}, Tokens: testTokens(t), Realtime: realtime.NewHub(), AllowedOrigins: []string{"https://hotel.example"}}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, response, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/v1/events", &websocket.DialOptions{
		Subprotocols: []string{realtimeProtocol}, HTTPHeader: http.Header{"Origin": []string{"https://attacker.example"}},
	})
	if err == nil {
		t.Fatal("untrusted origin must reject realtime connection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 response, got %+v", response)
	}
}

func TestRealtimeWebSocketRejectsMissingToken(t *testing.T) {
	server := httptest.NewServer(NewHandler(Dependencies{Store: &fakeStore{}, Tokens: testTokens(t), Realtime: realtime.NewHub(), AllowedOrigins: []string{"*"}}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, response, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/api/v1/events", &websocket.DialOptions{Subprotocols: []string{realtimeProtocol}})
	if err == nil {
		t.Fatal("missing token must reject realtime connection")
	}
	if response == nil || response.StatusCode != 401 {
		t.Fatalf("expected 401 response, got %+v", response)
	}
}
