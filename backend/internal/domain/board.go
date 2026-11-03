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
	Status    ColumnStatus
}

// ColumnStatus says what a column means, whatever it is called (OmiTeam statuses).
type ColumnStatus string

const (
	StatusBacklog    ColumnStatus = "backlog"
	StatusTodo       ColumnStatus = "todo"
	StatusInProgress ColumnStatus = "in_progress"
	StatusTesting    ColumnStatus = "testing"
	StatusDone       ColumnStatus = "done"
	StatusRejected   ColumnStatus = "rejected"
)

// StatusForName guesses a new column's status from its name ("To Do", "Done"…); it can be changed.
func StatusForName(name string) ColumnStatus {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "backlog", "беклог":
		return StatusBacklog
	case "to do", "todo", "to-do", "до виконання", "zu erledigen":
		return StatusTodo
	case "review", "testing", "qa", "тестування", "перевірка", "test":
		return StatusTesting
	case "done", "готово", "erledigt":
		return StatusDone
	case "rejected", "відхилено", "abgelehnt":
		return StatusRejected
	}
	return StatusInProgress
}

func (s ColumnStatus) Valid() bool {
	switch s {
	case StatusBacklog, StatusTodo, StatusInProgress, StatusTesting, StatusDone, StatusRejected:
		return true
	}
	return false
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

	TaskDetails
	IsTemplate     bool
	Tags           []Tag
	ChecklistDone  int
	ChecklistTotal int
	CommentCount   int
	TrackedMinutes int // time logged on the task
}

// TaskDetails are the OmiTeam task fields edited apart from the shared text.
type TaskDetails struct {
	StoryPoints *int
	BudgetHours *float64
	Billable    bool
	SpecialType SpecialType
	MilestoneID *int64
	SprintID    *int64
	VersionID   *int64
	TagIDs      []int64
}

// SpecialType marks non-project work logged as a task in the main project.
type SpecialType string

const (
	SpecialNone      SpecialType = ""
	SpecialSickDay   SpecialType = "sick_day"
	SpecialDayOff    SpecialType = "day_off"
	SpecialVacation  SpecialType = "vacation"
	SpecialPreSale   SpecialType = "pre_sale"
	SpecialBench     SpecialType = "bench"
	SpecialInterview SpecialType = "interview"
)

func (t SpecialType) Valid() bool {
	switch t {
	case SpecialNone, SpecialSickDay, SpecialDayOff, SpecialVacation, SpecialPreSale, SpecialBench, SpecialInterview:
		return true
	}
	return false
}

func (d TaskDetails) Validate() error {
	if d.StoryPoints != nil && (*d.StoryPoints < 0 || *d.StoryPoints > 1000) {
		return ErrTaskDetails
	}
	if d.BudgetHours != nil && *d.BudgetHours <= 0 {
		return ErrTaskDetails
	}
	if !d.SpecialType.Valid() || len(d.TagIDs) > 20 {
		return ErrTaskDetails
	}
	return nil
}

type Tag struct {
	ID        int64
	ProjectID int64
	Name      string
	Color     string
}

type Checklist struct {
	ID        int64
	CardID    int64
	Title     string
	Position  int
	Collapsed bool
	Items     []ChecklistItem
}

type ChecklistItem struct {
	ID          int64
	ChecklistID int64
	Text        string
	Done        bool
	Position    int
}

type CardComment struct {
	ID         int64
	CardID     int64
	AuthorID   *int64
	AuthorName string
	Body       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// HistoryEntry is one change of a task.
type HistoryEntry struct {
	ID        int64
	CardID    int64
	ActorID   *int64
	ActorName string
	Field     string // "title", "priority", "assignee", "column", "storyPoints", "tags"…
	Old, New  string
	CreatedAt time.Time
}

// --- planning ---

type Milestone struct {
	ID          int64
	ProjectID   int64
	Name        string
	Description string
	Start, End  *Date
	CardCount   int
	DoneCount   int
}

type SprintStatus string

const (
	SprintNew     SprintStatus = "new"
	SprintCurrent SprintStatus = "current"
	SprintPast    SprintStatus = "past"
)

type Sprint struct {
	ID         int64
	ProjectID  int64
	Name       string
	Goal       string
	Start, End *Date
	Status     SprintStatus
	CardCount  int
	DoneCount  int
}

type VersionStatus string

const (
	VersionInDevelopment VersionStatus = "in_development"
	VersionUpcoming      VersionStatus = "upcoming"
	VersionReleased      VersionStatus = "released"
	VersionArchived      VersionStatus = "archived"
)

type Version struct {
	ID          int64
	ProjectID   int64
	Name        string
	Description string
	ReleaseDate *Date
	Status      VersionStatus
	CardCount   int
	DoneCount   int
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
