package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/TablazOrg/HotelMate/backend/internal/auth"
	"github.com/TablazOrg/HotelMate/backend/internal/models"
)

type contextKey string

const (
	claimsKey contextKey = "authClaims"
	staffKey  contextKey = "authenticatedStaff"
	stayKey   contextKey = "authenticatedStay"
)

func (s *Server) require(actor auth.ActorType, roles ...models.StaffRole) func(http.Handler) http.Handler {
	allowedRoles := make(map[models.StaffRole]struct{}, len(roles))
	for _, role := range roles {
		allowedRoles[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.tokens == nil || s.store == nil {
				writeError(w, http.StatusServiceUnavailable, "authentication_unavailable", "سرویس احراز هویت در دسترس نیست")
				return
			}
			authorization := strings.Fields(r.Header.Get("Authorization"))
			if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
				return
			}
			claims, err := s.tokens.Parse(authorization[1], actor)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
				return
			}
			hotelID, _ := claims.TenantID()
			actorID, _ := claims.SubjectID()
			ctx := context.WithValue(r.Context(), claimsKey, claims)

			switch actor {
			case auth.ActorStaff:
				staff, err := s.store.FindStaffByID(ctx, hotelID, actorID)
				if err != nil || !staff.IsActive {
					writeError(w, http.StatusUnauthorized, "unauthorized", "نشست معتبر نیست")
					return
				}
				claims.Role = string(staff.Role)
				if len(allowedRoles) > 0 {
					if _, ok := allowedRoles[staff.Role]; !ok {
						writeError(w, http.StatusForbidden, "forbidden", "دسترسی کافی ندارید")
						return
					}
				}
				ctx = context.WithValue(ctx, staffKey, staff)
			case auth.ActorGuest:
				stayID, _ := claims.ActiveStayID()
				stay, err := s.store.FindGuestSession(ctx, hotelID, actorID, stayID)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "unauthorized", "اقامت فعال پیدا نشد")
					return
				}
				ctx = context.WithValue(ctx, stayKey, stay)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func currentClaims(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(claimsKey).(*auth.Claims)
	return claims
}

func currentStaff(r *http.Request) (models.StaffUser, bool) {
	staff, ok := r.Context().Value(staffKey).(models.StaffUser)
	return staff, ok
}

func currentStay(r *http.Request) (models.Stay, bool) {
	stay, ok := r.Context().Value(stayKey).(models.Stay)
	return stay, ok
}
