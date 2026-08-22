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

type fakeContentStore struct {
	content     store.HotelContent
	lastHotelID uuid.UUID
}

func (f *fakeContentStore) GetPublicContent(_ context.Context, _ string, _ time.Time) (store.HotelContent, error) {
	return f.content, nil
}
func (f *fakeContentStore) GetStaffContent(_ context.Context, hotelID uuid.UUID) (store.HotelContent, error) {
	f.lastHotelID = hotelID
	return f.content, nil
}
func (f *fakeContentStore) CreateFacility(_ context.Context, item *models.Facility) error {
	f.lastHotelID = item.HotelID
	item.ID = uuid.New()
	return nil
}
func (f *fakeContentStore) UpdateFacility(_ context.Context, hotelID, _ uuid.UUID, item models.Facility) (models.Facility, error) {
	f.lastHotelID = hotelID
	return item, nil
}
func (f *fakeContentStore) CreatePromotion(_ context.Context, item *models.Promotion) error {
	f.lastHotelID = item.HotelID
	item.ID = uuid.New()
	return nil
}
func (f *fakeContentStore) UpdatePromotion(_ context.Context, hotelID, _ uuid.UUID, item models.Promotion) (models.Promotion, error) {
	f.lastHotelID = hotelID
	return item, nil
}
func (f *fakeContentStore) CreateRestaurant(_ context.Context, item *models.Restaurant) error {
	f.lastHotelID = item.HotelID
	item.ID = uuid.New()
	return nil
}
func (f *fakeContentStore) UpdateRestaurant(_ context.Context, hotelID, _ uuid.UUID, item models.Restaurant) (models.Restaurant, error) {
	f.lastHotelID = hotelID
	return item, nil
}
func (f *fakeContentStore) CreateMenuItem(_ context.Context, hotelID, _ uuid.UUID, item *models.MenuItem) error {
	f.lastHotelID = hotelID
	item.ID = uuid.New()
	return nil
}
func (f *fakeContentStore) UpdateMenuItem(_ context.Context, hotelID, _ uuid.UUID, item models.MenuItem) (models.MenuItem, error) {
	f.lastHotelID = hotelID
	return item, nil
}

func TestPublicHotelContentIsAvailableWithoutAuthentication(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Public Hotel", Slug: "public-hotel"}
	content := &fakeContentStore{content: store.HotelContent{Hotel: hotel, Facilities: []models.Facility{{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Name: "استخر", Icon: "water", IsActive: true}}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/hotels/public-hotel/content", nil)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Content: content, AllowedOrigins: []string{"*"}}).ServeHTTP(res, req)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte("استخر")) {
		t.Fatalf("public content status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestContentMutationUsesAuthenticatedStaffTenant(t *testing.T) {
	hotel := models.Hotel{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Tenant Hotel", Slug: "tenant-hotel"}
	staff := models.StaffUser{BaseModel: models.BaseModel{ID: uuid.New()}, HotelID: hotel.ID, Hotel: hotel, Role: models.StaffRoleOperations, IsActive: true}
	content := &fakeContentStore{}
	tokens := testTokens(t)
	token, _, _ := tokens.IssueStaff(staff)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/facilities", bytes.NewBufferString(`{"name":"استخر","description":"","icon":"water","hours":"۷ تا ۲۳","sortOrder":10,"isActive":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	NewHandler(Dependencies{Store: &fakeStore{hotel: hotel, staff: staff}, Content: content, Tokens: tokens, AllowedOrigins: []string{"*"}}).ServeHTTP(res, req)
	if res.Code != http.StatusCreated || content.lastHotelID != hotel.ID {
		t.Fatalf("content mutation status=%d tenant=%s body=%s", res.Code, content.lastHotelID, res.Body.String())
	}
}

var _ store.ContentStore = (*fakeContentStore)(nil)
