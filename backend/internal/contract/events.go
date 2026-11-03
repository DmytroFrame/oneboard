// Package contract defines the messages that cross a process boundary: what server instances publish to
// each other over Redis pub/sub today, and what separate services would exchange tomorrow. These types
// are the inter-service API, documented language-neutrally in docs/contracts.md; domain entities stay
// free of serialization concerns.
//
// Field names are camelCase and, ignoring case, identical to the untagged encoding used before this
// package existed (Go matches JSON keys case-insensitively), so instances of both versions understand
// each other during a rolling deploy. Changing a name here is a breaking change for every consumer.
package contract

import (
	"encoding/json"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// BoardEnvelope travels on "board:<projectId>": either a committed BoardEvent, or a peer message
// (cursor, drag, document update…) relayed as the sending connection encoded it.
type BoardEnvelope struct {
	From    string          `json:"from,omitempty"` // sending connection, skipped when fanning a peer message out
	Event   *BoardEvent     `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// BoardEvent is a committed change on a board (see domain.BoardEventType for the types).
type BoardEvent struct {
	Type          string `json:"type"`
	ProjectID     int64  `json:"projectId"`
	ActorID       int64  `json:"actorId"`
	Origin        string `json:"origin,omitempty"` // browser tab that caused it; that tab skips the event
	Card          *Card  `json:"card,omitempty"`   // card.created, card.updated, card.moved
	CardID        int64  `json:"cardId,omitempty"` // card.deleted
	ColumnID      int64  `json:"columnId,omitempty"`
	FromColumnID  int64  `json:"fromColumnId,omitempty"` // card.moved
	AfterID       *int64 `json:"afterId,omitempty"`
	BeforeID      *int64 `json:"beforeId,omitempty"`
	RemovedUserID int64  `json:"removedUserId,omitempty"` // members.changed: this user lost access
}

type Card struct {
	ID              int64      `json:"id"`
	ProjectID       int64      `json:"projectId"`
	ColumnID        int64      `json:"columnId"`
	Key             string     `json:"key"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Priority        string     `json:"priority"`
	Position        int        `json:"position"`
	AuthorID        int64      `json:"authorId"`
	AssigneeID      *int64     `json:"assigneeId"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	AttachmentCount int        `json:"attachmentCount"`
	Cover           *CardCover `json:"cover"`

	// Task fields (added with OmiTeam's task management; absent from older publishers).
	StoryPoints    *int     `json:"storyPoints,omitempty"`
	BudgetHours    *float64 `json:"budgetHours,omitempty"`
	Billable       bool     `json:"billable"`
	SpecialType    string   `json:"specialType,omitempty"`
	MilestoneID    *int64   `json:"milestoneId,omitempty"`
	SprintID       *int64   `json:"sprintId,omitempty"`
	VersionID      *int64   `json:"versionId,omitempty"`
	IsTemplate     bool     `json:"isTemplate,omitempty"`
	Tags           []Tag    `json:"tags,omitempty"`
	ChecklistDone  int      `json:"checklistDone,omitempty"`
	ChecklistTotal int      `json:"checklistTotal,omitempty"`
	CommentCount   int      `json:"commentCount,omitempty"`
	TrackedMinutes int      `json:"trackedMinutes,omitempty"`
}

type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CardCover struct {
	AttachmentID int64  `json:"attachmentId"`
	Kind         string `json:"kind"`
	HasThumb     bool   `json:"hasThumb"`
}

// UserEvent travels on "user:<userId>" and reaches every tab that user has open.
type UserEvent struct {
	Type      string             `json:"type"`
	UserID    int64              `json:"userId"`
	Reason    string             `json:"reason,omitempty"`
	OrgID     int64              `json:"orgId,omitempty"`
	ProjectID int64              `json:"projectId,omitempty"`
	Name      string             `json:"name,omitempty"`
	ActorID   int64              `json:"actorId,omitempty"`
	Voice     []VoiceParticipant `json:"voice,omitempty"` // type "voice": everyone now in the room
}

type VoiceParticipant struct {
	ConnID string `json:"connId"`
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
	Muted  bool   `json:"muted"`
}

// ---------- domain ↔ contract ----------

func NewBoardEvent(e domain.BoardEvent) BoardEvent {
	out := BoardEvent{
		Type: string(e.Type), ProjectID: e.ProjectID, ActorID: e.ActorID, Origin: e.Origin,
		CardID: e.CardID, ColumnID: e.ColumnID, FromColumnID: e.FromColumnID,
		AfterID: e.AfterID, BeforeID: e.BeforeID, RemovedUserID: e.RemovedUserID,
	}
	if c := e.Card; c != nil {
		card := Card{
			ID: c.ID, ProjectID: c.ProjectID, ColumnID: c.ColumnID, Key: c.Key, Title: c.Title,
			Description: c.Description, Priority: string(c.Priority), Position: c.Position,
			AuthorID: c.AuthorID, AssigneeID: c.AssigneeID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
			AttachmentCount: c.AttachmentCount,
			StoryPoints:     c.StoryPoints, BudgetHours: c.BudgetHours, Billable: c.Billable, SpecialType: string(c.SpecialType),
			MilestoneID: c.MilestoneID, SprintID: c.SprintID, VersionID: c.VersionID, IsTemplate: c.IsTemplate,
			ChecklistDone: c.ChecklistDone, ChecklistTotal: c.ChecklistTotal, CommentCount: c.CommentCount, TrackedMinutes: c.TrackedMinutes,
		}
		for _, t := range c.Tags {
			card.Tags = append(card.Tags, Tag{t.ID, t.Name, t.Color})
		}
		if cv := c.Cover; cv != nil {
			card.Cover = &CardCover{AttachmentID: cv.AttachmentID, Kind: string(cv.Kind), HasThumb: cv.HasThumb}
		}
		out.Card = &card
	}
	return out
}

func (e BoardEvent) Domain() domain.BoardEvent {
	out := domain.BoardEvent{
		Type: domain.BoardEventType(e.Type), ProjectID: e.ProjectID, ActorID: e.ActorID, Origin: e.Origin,
		CardID: e.CardID, ColumnID: e.ColumnID, FromColumnID: e.FromColumnID,
		AfterID: e.AfterID, BeforeID: e.BeforeID, RemovedUserID: e.RemovedUserID,
	}
	if c := e.Card; c != nil {
		card := domain.Card{
			ID: c.ID, ProjectID: c.ProjectID, ColumnID: c.ColumnID, Key: c.Key, Title: c.Title,
			Description: c.Description, Priority: domain.Priority(c.Priority), Position: c.Position,
			AuthorID: c.AuthorID, AssigneeID: c.AssigneeID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
			AttachmentCount: c.AttachmentCount,
			TaskDetails: domain.TaskDetails{StoryPoints: c.StoryPoints, BudgetHours: c.BudgetHours, Billable: c.Billable,
				SpecialType: domain.SpecialType(c.SpecialType), MilestoneID: c.MilestoneID, SprintID: c.SprintID, VersionID: c.VersionID},
			IsTemplate: c.IsTemplate, ChecklistDone: c.ChecklistDone, ChecklistTotal: c.ChecklistTotal,
			CommentCount: c.CommentCount, TrackedMinutes: c.TrackedMinutes,
		}
		for _, t := range c.Tags {
			card.Tags = append(card.Tags, domain.Tag{ID: t.ID, ProjectID: c.ProjectID, Name: t.Name, Color: t.Color})
			card.TagIDs = append(card.TagIDs, t.ID)
		}
		if cv := c.Cover; cv != nil {
			card.Cover = &domain.CardCover{AttachmentID: cv.AttachmentID, Kind: domain.AttachmentKind(cv.Kind), HasThumb: cv.HasThumb}
		}
		out.Card = &card
	}
	return out
}

func NewUserEvent(e domain.UserEvent) UserEvent {
	out := UserEvent{
		Type: string(e.Type), UserID: e.UserID, Reason: string(e.Reason), OrgID: e.OrgID,
		ProjectID: e.ProjectID, Name: e.Name, ActorID: e.ActorID,
	}
	for _, p := range e.Voice {
		out.Voice = append(out.Voice, VoiceParticipant(p))
	}
	return out
}

func (e UserEvent) Domain() domain.UserEvent {
	out := domain.UserEvent{
		Type: domain.UserEventType(e.Type), UserID: e.UserID, Reason: domain.AccessReason(e.Reason), OrgID: e.OrgID,
		ProjectID: e.ProjectID, Name: e.Name, ActorID: e.ActorID,
	}
	for _, p := range e.Voice {
		out.Voice = append(out.Voice, domain.VoiceParticipant(p))
	}
	return out
}
