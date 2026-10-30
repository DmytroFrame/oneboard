package httpapi

import (
	"fmt"
	"time"

	"clodapp/backend/internal/domain"
)

// DTOs keep the JSON contract in the transport layer; domain entities carry no tags.

type userDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Color     string  `json:"color"`
	AvatarURL *string `json:"avatarUrl"`
}

type settingsDTO struct {
	Language domain.Language `json:"language"`
	Theme    domain.Theme    `json:"theme"`
}

// meDTO is the signed-in user; settings are private and only ever returned to their owner.
type meDTO struct {
	userDTO
	Settings settingsDTO `json:"settings"`
}

type memberDTO struct {
	userDTO
	Role          domain.Role `json:"role"`
	AssignedCount int         `json:"assignedCount"`
}

type projectDTO struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Key         string      `json:"key"`
	OwnerID     int64       `json:"ownerId"`
	Role        domain.Role `json:"role"`
	MemberCount int         `json:"memberCount"`
	CardCount   int         `json:"cardCount"`
	CreatedAt   time.Time   `json:"createdAt"`
}

type columnDTO struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	CardCount int    `json:"cardCount"`
}

type cardDTO struct {
	ID          int64           `json:"id"`
	ProjectID   int64           `json:"projectId"`
	ColumnID    int64           `json:"columnId"`
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Priority    domain.Priority `json:"priority"`
	Position    int             `json:"position"`
	AuthorID    int64           `json:"authorId"`
	AssigneeID  *int64          `json:"assigneeId"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// sessionDTO answers login, registration and refresh. The refresh token travels only in its HttpOnly cookie.
type sessionDTO struct {
	User        meDTO     `json:"user"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type cardPageDTO struct {
	Cards      []cardDTO `json:"cards"`
	NextCursor *string   `json:"nextCursor"` // "position:id"; null when there are no more cards
	Total      int       `json:"total"`
}

func toProject(p domain.Project) projectDTO {
	return projectDTO{p.ID, p.Name, p.Key, p.OwnerID, p.Role, p.MemberCount, p.CardCount, p.CreatedAt}
}

func toColumns(cs []domain.Column) []columnDTO {
	out := make([]columnDTO, len(cs))
	for i, c := range cs {
		out[i] = columnDTO{c.ID, c.ProjectID, c.Name, c.Position, c.CardCount}
	}
	return out
}

func toCard(c domain.Card) cardDTO {
	return cardDTO{c.ID, c.ProjectID, c.ColumnID, c.Key, c.Title, c.Description, c.Priority, c.Position,
		c.AuthorID, c.AssigneeID, c.CreatedAt, c.UpdatedAt}
}

func toCardPage(p domain.CardPage) cardPageDTO {
	out := cardPageDTO{Cards: make([]cardDTO, len(p.Cards)), Total: p.Total}
	for i, c := range p.Cards {
		out.Cards[i] = toCard(c)
	}
	if p.NextCursor != nil {
		s := fmt.Sprintf("%d:%d", p.NextCursor.Position, p.NextCursor.ID)
		out.NextCursor = &s
	}
	return out
}

func mapSlice[T, R any](in []T, f func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
