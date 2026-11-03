package httpapi

import (
	"fmt"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the cards module.

type coverDTO struct {
	Kind domain.AttachmentKind `json:"kind"`
	URL  string                `json:"url"`
}

type cardDTO struct {
	ID              int64           `json:"id"`
	ProjectID       int64           `json:"projectId"`
	ColumnID        int64           `json:"columnId"`
	Key             string          `json:"key"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Priority        domain.Priority `json:"priority"`
	Position        int             `json:"position"`
	AuthorID        int64           `json:"authorId"`
	AssigneeID      *int64          `json:"assigneeId"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	AttachmentCount int             `json:"attachmentCount"`
	Cover           *coverDTO       `json:"cover"` // first image or video, shown on the card face

	StoryPoints    *int               `json:"storyPoints"`
	BudgetHours    *float64           `json:"budgetHours"`
	Billable       bool               `json:"billable"`
	SpecialType    domain.SpecialType `json:"specialType"`
	MilestoneID    *int64             `json:"milestoneId"`
	SprintID       *int64             `json:"sprintId"`
	VersionID      *int64             `json:"versionId"`
	IsTemplate     bool               `json:"isTemplate"`
	Tags           []tagDTO           `json:"tags"`
	ChecklistDone  int                `json:"checklistDone"`
	ChecklistTotal int                `json:"checklistTotal"`
	CommentCount   int                `json:"commentCount"`
	TrackedMinutes int                `json:"trackedMinutes"`
}

type tagDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func toTag(t domain.Tag) tagDTO { return tagDTO{t.ID, t.Name, t.Color} }

type cardPageDTO struct {
	Cards      []cardDTO `json:"cards"`
	NextCursor *string   `json:"nextCursor"` // "position:id"; null when there are no more cards
	Total      int       `json:"total"`
}

func (p *Presenter) Card(c domain.Card) cardDTO {
	d := cardDTO{ID: c.ID, ProjectID: c.ProjectID, ColumnID: c.ColumnID, Key: c.Key, Title: c.Title, Description: c.Description,
		Priority: c.Priority, Position: c.Position, AuthorID: c.AuthorID, AssigneeID: c.AssigneeID, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt, AttachmentCount: c.AttachmentCount,
		StoryPoints: c.StoryPoints, BudgetHours: c.BudgetHours, Billable: c.Billable, SpecialType: c.SpecialType,
		MilestoneID: c.MilestoneID, SprintID: c.SprintID, VersionID: c.VersionID, IsTemplate: c.IsTemplate,
		Tags: mapSlice(c.Tags, toTag), ChecklistDone: c.ChecklistDone, ChecklistTotal: c.ChecklistTotal,
		CommentCount: c.CommentCount, TrackedMinutes: c.TrackedMinutes}
	if cv := c.Cover; cv != nil {
		variant := "file"
		if cv.HasThumb {
			variant = "thumb"
		}
		d.Cover = &coverDTO{Kind: cv.Kind, URL: p.attachmentURL(cv.AttachmentID, variant)}
	}
	return d
}

func (p *Presenter) CardPage(page domain.CardPage) cardPageDTO {
	return cardPageDTO{Cards: mapSlice(page.Cards, p.Card), NextCursor: cursorString(page.NextCursor), Total: page.Total}
}

func cursorString(c *domain.CardCursor) *string {
	if c == nil {
		return nil
	}
	s := fmt.Sprintf("%d:%d", c.Position, c.ID)
	return &s
}
