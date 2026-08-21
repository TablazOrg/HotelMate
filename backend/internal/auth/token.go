package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ActorType string

const (
	ActorStaff ActorType = "staff"
	ActorGuest ActorType = "guest"
)

const (
	staffAudience = "hotelmate:staff"
	guestAudience = "hotelmate:guest"
)

type Claims struct {
	HotelID   string    `json:"hotelId"`
	ActorType ActorType `json:"actorType"`
	Role      string    `json:"role,omitempty"`
	StayID    string    `json:"stayId,omitempty"`
	jwt.RegisteredClaims
}

func (c Claims) SubjectID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

func (c Claims) TenantID() (uuid.UUID, error) {
	return uuid.Parse(c.HotelID)
}

func (c Claims) ActiveStayID() (uuid.UUID, error) {
	if c.StayID == "" {
		return uuid.Nil, errors.New("stay ID is missing")
	}
	return uuid.Parse(c.StayID)
}

type TokenManager struct {
	secret   []byte
	issuer   string
	staffTTL time.Duration
	guestTTL time.Duration
	now      func() time.Time
}

func NewTokenManager(secret, issuer string, staffTTL, guestTTL time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 characters")
	}
	if issuer == "" {
		return nil, errors.New("JWT issuer is required")
	}
	if staffTTL <= 0 || guestTTL <= 0 {
		return nil, errors.New("token TTL values must be positive")
	}
	return &TokenManager{secret: []byte(secret), issuer: issuer, staffTTL: staffTTL, guestTTL: guestTTL, now: time.Now}, nil
}

func (m *TokenManager) IssueStaff(staff models.StaffUser) (string, time.Time, error) {
	return m.issue(staff.ID, staff.HotelID, ActorStaff, string(staff.Role), uuid.Nil, staffAudience, m.staffTTL)
}

func (m *TokenManager) IssueGuest(stay models.Stay) (string, time.Time, error) {
	return m.issue(stay.GuestID, stay.HotelID, ActorGuest, "", stay.ID, guestAudience, m.guestTTL)
}

func (m *TokenManager) issue(subject, hotelID uuid.UUID, actor ActorType, role string, stayID uuid.UUID, audience string, ttl time.Duration) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(ttl)
	claims := Claims{
		HotelID:   hotelID.String(),
		ActorType: actor,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject.String(),
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	if stayID != uuid.Nil {
		claims.StayID = stayID.String()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(tokenString string, expectedActor ActorType) (*Claims, error) {
	audience, err := audienceFor(expectedActor)
	if err != nil {
		return nil, err
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	if claims.ActorType != expectedActor {
		return nil, errors.New("token actor type does not match endpoint")
	}
	if _, err := claims.SubjectID(); err != nil {
		return nil, errors.New("token subject is invalid")
	}
	if _, err := claims.TenantID(); err != nil {
		return nil, errors.New("token hotel is invalid")
	}
	if expectedActor == ActorGuest {
		if _, err := claims.ActiveStayID(); err != nil {
			return nil, errors.New("guest token stay is invalid")
		}
	}
	return claims, nil
}

func audienceFor(actor ActorType) (string, error) {
	switch actor {
	case ActorStaff:
		return staffAudience, nil
	case ActorGuest:
		return guestAudience, nil
	default:
		return "", errors.New("unsupported actor type")
	}
}
