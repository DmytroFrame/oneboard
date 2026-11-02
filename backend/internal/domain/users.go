package domain

import "time"

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
