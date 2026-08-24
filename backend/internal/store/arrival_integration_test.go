package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/database"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/TablazOrg/HotelMate/backend/internal/store"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestArrivalJourneyInvitationCorrectionAndReadinessPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close(db)
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repository := store.New(db)
	suffix := uuid.NewString()[:8]
	hotel := models.Hotel{Name: "Arrival " + suffix, Slug: "arrival-" + suffix, PrimaryColor: "#0f766e", Timezone: "UTC"}
	if err := db.Create(&hotel).Error; err != nil {
		t.Fatalf("create hotel: %v", err)
	}
	settings := models.DefaultArrivalSettings(hotel.ID)
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}
	staff := models.StaffUser{HotelID: hotel.ID, FirstName: "Reza", LastName: "Desk", Email: suffix + "@example.com", PasswordHash: "unused", Role: models.StaffRoleReception, IsActive: true}
	if err := db.Create(&staff).Error; err != nil {
		t.Fatalf("create staff: %v", err)
	}
	room := models.Room{HotelID: hotel.ID, Number: "A-" + suffix, Status: models.RoomStatusAvailable}
	if err := repository.CreateRoom(ctx, &room); err != nil {
		t.Fatalf("create room: %v", err)
	}
	identityHash, _ := auth.HashIdentity("ARRIVAL-TEST-" + suffix)
	guest := models.Guest{FirstName: "Mina", LastName: "Guest", IdentityType: "passport", IdentityNumberHash: identityHash, Phone: "+989120000000"}
	arrivalDate := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	reservation := models.Reservation{HotelID: hotel.ID, RoomID: &room.ID, ConfirmationCode: "A" + suffix, Status: models.ReservationPending, ArrivalDate: arrivalDate, DepartureDate: arrivalDate.Add(48 * time.Hour)}
	if err := repository.CreateReservation(ctx, &guest, &reservation); err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	_, stay, err := repository.ConfirmReservation(ctx, hotel.ID, reservation.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	if _, err := repository.CreateCheckInInvitation(ctx, hotel.ID, reservation.ID, staff.ID, store.HashInvitationSecret("disabled"), store.HashInvitationSecret("disabled-recovery"), time.Now().Add(time.Hour), time.Now()); !errors.Is(err, store.ErrArrivalDisabled) {
		t.Fatalf("disabled feature must fail closed, got %v", err)
	}
	settings.OnlineCheckInEnabled, settings.DigitalRegistrationEnabled = true, true
	if _, err := repository.UpdateArrivalSettings(ctx, hotel.ID, settings); err != nil {
		t.Fatalf("enable settings: %v", err)
	}

	inviteToken, recovery := "signed-invitation-"+suffix, "RECOVERY-"+suffix
	invitation, err := repository.CreateCheckInInvitation(ctx, hotel.ID, reservation.ID, staff.ID, store.HashInvitationSecret(inviteToken), store.HashInvitationSecret(recovery), time.Now().Add(time.Hour), time.Now())
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	_, redeemedStay, journey, _, err := repository.RedeemCheckInInvitation(ctx, store.HashInvitationSecret(inviteToken), "", &reservation.ID, &hotel.ID, time.Now())
	if err != nil {
		t.Fatalf("redeem invitation: %v", err)
	}
	if redeemedStay.ID != stay.ID || journey.Status != models.ArrivalDraft || journey.PaymentStep == nil {
		t.Fatalf("unexpected redeemed journey: stay=%s journey=%+v", redeemedStay.ID, journey)
	}

	eta := arrivalDate.Add(2 * time.Hour)
	journey, err = repository.SaveArrivalDetails(ctx, hotel.ID, stay.ID, store.ArrivalDetailsInput{ContactPhone: "+989120000000", ContactEmail: "mina@example.com", Nationality: "IR", ArrivalETA: &eta, ArrivalMethod: "taxi", AnswersJSON: []byte(`{"quietRoom":true}`)}, time.Now())
	if err != nil {
		t.Fatalf("save details: %v", err)
	}
	journey, err = repository.ReplaceArrivalCompanions(ctx, hotel.ID, stay.ID, []models.ArrivalCompanion{{FirstName: "Sara", LastName: "Guest", Relationship: "child", DocumentRequired: true}}, time.Now())
	if err != nil || len(journey.Companions) != 1 {
		t.Fatalf("save companion: %+v %v", journey.Companions, err)
	}
	retention := reservation.DepartureDate.Add(30 * 24 * time.Hour)
	if _, err := repository.AddArrivalDocument(ctx, hotel.ID, stay.ID, models.ArrivalDocument{EvidenceType: "passport", Side: "single", StorageKey: "check-in/guest-" + suffix, Name: "guest.png", MediaType: "image/png", Size: 100, SHA256: "guest", VerificationState: "manual_review", RetentionUntil: retention}, time.Now()); err != nil {
		t.Fatalf("add guest document: %v", err)
	}
	journey, _, err = repository.SaveArrivalSignature(ctx, hotel.ID, stay.ID, models.ArrivalJourney{TermsVersion: settings.TermsVersion, TermsLocale: settings.TermsLocale, SignerName: "Mina Guest", SignatureStorageKey: "check-in/signature-" + suffix, SignatureMediaType: "image/png", SignatureSize: 120, SignatureSHA256: "signature"}, time.Now())
	if err != nil {
		t.Fatalf("save signature: %v", err)
	}
	if _, err := repository.SubmitArrival(ctx, hotel.ID, stay.ID, "idempotency-1", time.Now()); !errors.Is(err, store.ErrArrivalIncomplete) {
		t.Fatalf("missing companion evidence must block submission, got %v", err)
	}
	companionID := journey.Companions[0].ID
	if _, err := repository.AddArrivalDocument(ctx, hotel.ID, stay.ID, models.ArrivalDocument{CompanionID: &companionID, EvidenceType: "identity", Side: "front", StorageKey: "check-in/companion-" + suffix, Name: "companion.png", MediaType: "image/png", Size: 100, SHA256: "companion", VerificationState: "manual_review", RetentionUntil: retention}, time.Now()); err != nil {
		t.Fatalf("add companion document: %v", err)
	}
	settings.TermsVersion = "v2-" + suffix
	if _, err := repository.UpdateArrivalSettings(ctx, hotel.ID, settings); err != nil {
		t.Fatalf("rotate terms: %v", err)
	}
	if _, err := repository.SubmitArrival(ctx, hotel.ID, stay.ID, "outdated-terms", time.Now()); !errors.Is(err, store.ErrTermsOutdated) {
		t.Fatalf("outdated signature must block submission, got %v", err)
	}
	journey, replacedSignatureKey, err := repository.SaveArrivalSignature(ctx, hotel.ID, stay.ID, models.ArrivalJourney{TermsVersion: settings.TermsVersion, TermsLocale: settings.TermsLocale, SignerName: "Mina Guest", SignatureStorageKey: "check-in/signature-v2-" + suffix, SignatureMediaType: "image/png", SignatureSize: 121, SignatureSHA256: "signature-v2"}, time.Now())
	if err != nil || replacedSignatureKey != "check-in/signature-"+suffix {
		t.Fatalf("replace signature for current terms: old=%q err=%v", replacedSignatureKey, err)
	}
	otherHotel := models.Hotel{Name: "Other " + suffix, Slug: "other-" + suffix, PrimaryColor: "#334155", Timezone: "UTC"}
	if err := db.Create(&otherHotel).Error; err != nil {
		t.Fatalf("create isolation hotel: %v", err)
	}
	if _, err := repository.GetArrivalDocument(ctx, otherHotel.ID, journey.ID, journey.Documents[0].ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant evidence read must not resolve, got %v", err)
	}
	submitted, err := repository.SubmitArrival(ctx, hotel.ID, stay.ID, "idempotency-2", time.Now())
	if err != nil || submitted.Status != models.ArrivalSubmitted || submitted.CompletenessScore != 100 {
		t.Fatalf("submit: %+v %v", submitted, err)
	}
	replayed, err := repository.SubmitArrival(ctx, hotel.ID, stay.ID, "idempotency-2", time.Now())
	if err != nil || replayed.Status != models.ArrivalSubmitted {
		t.Fatalf("idempotent replay: %+v %v", replayed, err)
	}

	needsChanges, err := repository.ReviewArrival(ctx, hotel.ID, journey.ID, staff.ID, models.ArrivalNeedsChanges, "تصویر پشت مدرک لازم است", time.Now())
	if err != nil || needsChanges.Status != models.ArrivalNeedsChanges {
		t.Fatalf("request changes: %+v %v", needsChanges, err)
	}
	resubmitted, err := repository.SubmitArrival(ctx, hotel.ID, stay.ID, "idempotency-3", time.Now())
	if err != nil || resubmitted.Status != models.ArrivalSubmitted {
		t.Fatalf("resubmit: %+v %v", resubmitted, err)
	}
	approved, err := repository.ReviewArrival(ctx, hotel.ID, journey.ID, staff.ID, models.ArrivalApproved, "", time.Now())
	if err != nil || approved.Status != models.ArrivalApproved {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	if _, err := repository.TransitionArrival(ctx, hotel.ID, journey.ID, staff.ID, models.ArrivalPending, time.Now()); err != nil {
		t.Fatalf("arrival pending: %v", err)
	}
	ready, err := repository.TransitionArrival(ctx, hotel.ID, journey.ID, staff.ID, models.ArrivalRoomReady, time.Now())
	if err != nil || ready.Status != models.ArrivalRoomReady {
		t.Fatalf("room ready: %+v %v", ready, err)
	}
	if _, err := repository.CheckInStay(ctx, hotel.ID, stay.ID, room.ID, time.Now()); err != nil {
		t.Fatalf("physical check-in: %v", err)
	}
	checkedIn, _, err := repository.GetGuestArrival(ctx, hotel.ID, stay.ID)
	if err != nil || checkedIn.Status != models.ArrivalCheckedIn {
		t.Fatalf("journey not closed by physical arrival: %+v %v", checkedIn, err)
	}
	analytics, err := repository.ArrivalAnalytics(ctx, hotel.ID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil || analytics.Invitations < 1 || analytics.Submitted < 2 || analytics.Approved < 1 || analytics.CheckedIn < 1 {
		t.Fatalf("arrival analytics incomplete: %+v %v", analytics, err)
	}

	expiredToken := "expired-invitation-" + suffix
	if _, err := repository.CreateCheckInInvitation(ctx, hotel.ID, reservation.ID, staff.ID, store.HashInvitationSecret(expiredToken), store.HashInvitationSecret("expired-recovery-"+suffix), time.Now().Add(-time.Minute), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("create expired invitation fixture: %v", err)
	}
	if _, _, _, _, err := repository.RedeemCheckInInvitation(ctx, store.HashInvitationSecret(expiredToken), "", &reservation.ID, &hotel.ID, time.Now()); !errors.Is(err, store.ErrInvitationExpired) {
		t.Fatalf("expired invitation must fail, got %v", err)
	}

	if err := repository.RevokeCheckInInvitation(ctx, hotel.ID, invitation.ID, staff.ID, time.Now()); err != nil {
		t.Fatalf("revoke invitation: %v", err)
	}
	if _, _, _, _, err := repository.RedeemCheckInInvitation(ctx, store.HashInvitationSecret(inviteToken), "", &reservation.ID, &hotel.ID, time.Now()); !errors.Is(err, store.ErrInvitationRevoked) {
		t.Fatalf("revoked invitation must fail, got %v", err)
	}
}
