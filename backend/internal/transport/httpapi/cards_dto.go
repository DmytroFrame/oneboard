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
}

type cardPageDTO struct {
	Cards      []cardDTO `json:"cards"`
	NextCursor *string   `json:"nextCursor"` // "position:id"; null when there are no more cards
	Total      int       `json:"total"`
}

func (p *Presenter) Card(c domain.Card) cardDTO {
	d := cardDTO{c.ID, c.ProjectID, c.ColumnID, c.Key, c.Title, c.Description, c.Priority, c.Position,
		c.AuthorID, c.AssigneeID, c.CreatedAt, c.UpdatedAt, c.AttachmentCount, nil}
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
