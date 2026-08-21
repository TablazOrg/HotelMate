package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const passwordCost = 12

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must contain at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashIdentity(identityNumber string) (string, error) {
	normalized := NormalizeIdentity(identityNumber)
	if len(normalized) < 4 {
		return "", errors.New("identity number is too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(normalized), passwordCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyIdentity(hash, identityNumber string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(NormalizeIdentity(identityNumber))) == nil
}

// NormalizeIdentity accepts common human formatting without exposing or
// retaining the normalized identifier outside password comparison.
func NormalizeIdentity(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return unicode.ToUpper(r)
	}, strings.TrimSpace(value))
}
