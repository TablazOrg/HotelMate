package store

import (
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
)

func TestServiceAvailableAtUsesHotelDailyWindow(t *testing.T) {
	day := models.Service{AvailableFrom: "08:00", AvailableUntil: "22:00"}
	if !serviceAvailableAt(day, "UTC", time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("midday service should be available")
	}
	if serviceAvailableAt(day, "UTC", time.Date(2026, 8, 22, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("late service should be unavailable")
	}

	overnight := models.Service{AvailableFrom: "22:00", AvailableUntil: "04:00"}
	if !serviceAvailableAt(overnight, "UTC", time.Date(2026, 8, 22, 1, 0, 0, 0, time.UTC)) {
		t.Fatal("overnight window must cross midnight")
	}
	if serviceAvailableAt(overnight, "UTC", time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("overnight service should be closed at noon")
	}
}
