package domain

import (
	"strings"
	"time"
	"unicode"
)

// Users and their interface preferences.

type User struct {
	ID              int64
	Name            string
	Email           string
	Color           string
	PasswordHash    string
	AvatarKey       string     // object key in the avatar storage; "" = no avatar
	AvatarUpdatedAt *time.Time // used to version the avatar URL for caching
	Settings        Settings
	CreatedAt       time.Time

	// The person's membership in OrgID, the organization the request works in (0 = none): their
	// company role there (see company.go) and whether they still work there. Organizations are
	// isolated from each other, so all of this is per organization.
	OrgID           int64
	Role            CompanyRole
	AdditionRoles   []AdditionRole // e.g. LEAD
	Active          bool           // deactivated and dismissed members keep their data but lose access
	DismissedAt     *time.Time
	DismissalReason string

	// PlatformAdmin runs the installation (backups, database schema); set from PLATFORM_ADMINS, not a
	// role in any organization.
	PlatformAdmin bool
}

// Membership is someone's place in an organization.
type Membership struct {
	OrgID           int64
	OrgName         string
	Role            CompanyRole
	AdditionRoles   []AdditionRole
	Active          bool
	DismissedAt     *time.Time
	DismissalReason string
	AddedAt         time.Time
}

// In returns the user as a member of m's organization. An inactive membership grants nothing.
func (u User) In(m Membership) User {
	u.OrgID, u.Role, u.AdditionRoles, u.Active, u.DismissedAt, u.DismissalReason =
		m.OrgID, m.Role, m.AdditionRoles, m.Active, m.DismissedAt, m.DismissalReason
	if !m.Active {
		u.Role, u.AdditionRoles = "", nil
	}
	return u
}

// Settings are the user's interface preferences, stored with the account so they follow them across devices.
type Settings struct {
	Language Language
	Theme    Theme
}

type Language string

const (
	LanguageAuto Language = "auto" // follow the browser
	LanguageEN   Language = "en"
	LanguageUK   Language = "uk"
	LanguageDE   Language = "de"
)

type Theme string

const (
	ThemeAuto  Theme = "auto" // follow the OS
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

func (s Settings) Validate() error {
	switch s.Language {
	case LanguageAuto, LanguageEN, LanguageUK, LanguageDE:
	default:
		return ErrLanguageInvalid
	}
	switch s.Theme {
	case ThemeAuto, ThemeLight, ThemeDark:
	default:
		return ErrThemeInvalid
	}
	return nil
}

func (u User) HasAvatar() bool { return u.AvatarKey != "" }

// AvatarColors are assigned round-robin to new users for their initials badge.
var AvatarColors = []string{
	"#0052CC", "#00875A", "#FF5630", "#6554C0", "#FF991F", "#00B8D9", "#36B37E", "#DE350B",
}

// ValidatePassword enforces the password policy for every new password: 8–72 characters (bcrypt's
// limit) with an upper-case and a lower-case letter, a digit and a special character.
func ValidatePassword(p string) error {
	if len(p) < 8 || len(p) > 72 {
		return ErrPasswordInvalid
	}
	var upper, lower, digit, special bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsSpace(r):
			special = true
		}
	}
	if !upper || !lower || !digit || !special {
		return ErrPasswordWeak
	}
	return nil
}

// NormalizeEmail is how emails are stored and compared.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
