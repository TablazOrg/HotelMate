package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
)

type arrivalExchangeSpy struct {
	store.ArrivalStore
	tokenHash    string
	recoveryHash string
}

func (s *arrivalExchangeSpy) RedeemCheckInInvitation(_ context.Context, tokenHash, recoveryHash string, _ *uuid.UUID, _ *uuid.UUID, _ time.Time) (models.CheckInInvitation, models.Stay, models.ArrivalJourney, models.ArrivalSettings, error) {
	s.tokenHash, s.recoveryHash = tokenHash, recoveryHash
	return models.CheckInInvitation{}, models.Stay{}, models.ArrivalJourney{}, models.ArrivalSettings{}, store.ErrInvitationRevoked
}

func TestRecoveryExchangeHashesOnlyTheSelectedCapability(t *testing.T) {
	arrival := &arrivalExchangeSpy{}
	handler := NewHandler(Dependencies{Arrival: arrival, Tokens: testTokens(t), Version: "test", AllowedOrigins: []string{"*"}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/check-in/exchange", bytes.NewBufferString(`{"recoveryCode":"abcd2345"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if arrival.tokenHash != "" {
		t.Fatalf("empty invitation token must stay empty, got hash %q", arrival.tokenHash)
	}
	if want := store.HashInvitationSecret("ABCD2345"); arrival.recoveryHash != want {
		t.Fatalf("recovery hash = %q, want %q", arrival.recoveryHash, want)
	}
}
