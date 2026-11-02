package domain

import (
	"strings"
	"time"
)

// The board: columns and cards.

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
