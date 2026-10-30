package domain

import (
	"strings"
	"time"
)

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

type Role string

const (
	RoleNone   Role = ""
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type Project struct {
	ID          int64
	OrgID       int64
	OrgName     string
	Name        string
	Key         string
	OwnerID     int64
	Role        Role // the current user's role
	MemberCount int
	CardCount   int
	CreatedAt   time.Time
}

type Member struct {
	User
	Role          Role
	AssignedCount int // cards assigned to them in the project
}

// DefaultColumns are created for every new project.
var DefaultColumns = []string{"To Do", "In Progress", "Review", "Done"}

const MaxColumnsPerProject = 20

type Column struct {
	ID        int64
	ProjectID int64
	Name      string
	Position  int
	CardCount int
}

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	}
	return false
}

type Card struct {
	ID          int64
	ProjectID   int64
	ColumnID    int64
	Key         string // e.g. KAN-12
	Title       string
	Description string
	Priority    Priority
	Position    int
	AuthorID    int64
	AssigneeID  *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time

	AttachmentCount int
	Cover           *CardCover
}

// CardDraft is the editable part of a card, validated by Normalize.
type CardDraft struct {
	Title       string
	Description string
	Priority    Priority
	AssigneeID  *int64
}

func (d *CardDraft) Normalize() error {
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" || len(d.Title) > 200 {
		return ErrCardTitle
	}
	if len(d.Description) > 10000 {
		return ErrCardDescription
	}
	if d.Priority == "" {
		d.Priority = PriorityMedium
	}
	if !d.Priority.Valid() {
		return ErrPriorityInvalid
	}
	return nil
}

// CardCursor is a keyset pagination position within a column.
type CardCursor struct {
	Position int
	ID       int64
}

type CardPage struct {
	Cards      []Card
	NextCursor *CardCursor // nil when everything has been returned
	Total      int         // matching cards in the column, ignoring the cursor
}

// SearchDoc is what the full-text index stores for a card.
type SearchDoc struct {
	ID          int64
	ProjectID   int64
	Key         string
	Title       string
	Description string
}
