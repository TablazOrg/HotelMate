package httpapi

import (
	"net/mail"
	"regexp"
	"strings"

	"github.com/TablazOrg/HotelMate/backend/internal/models"
)

var (
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func normalizeSlug(value string) string  { return strings.ToLower(strings.TrimSpace(value)) }
func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func normalizeRoom(value string) string  { return strings.ToUpper(strings.TrimSpace(value)) }

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && len(value) <= 254
}

func validRole(role models.StaffRole) bool {
	switch role {
	case models.StaffRolePrimaryAdmin,
		models.StaffRoleSecondaryAdmin,
		models.StaffRoleOperations,
		models.StaffRoleReception,
		models.StaffRoleHousekeeping,
		models.StaffRoleFB:
		return true
	default:
		return false
	}
}
