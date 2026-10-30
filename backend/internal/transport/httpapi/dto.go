package httpapi

import (
	"fmt"
	"time"

	"clodapp/backend/internal/domain"
)

// DTOs keep the JSON contract in the transport layer; domain entities carry no tags.
// Types with URLs (users, cards, attachments) are built by Presenter, which signs them.

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

type orgDTO struct {
	ID           int64          `json:"id"`
	Name         string         `json:"name"`
	Role         domain.OrgRole `json:"role"`
	MemberCount  int            `json:"memberCount"`
	ProjectCount int            `json:"projectCount"`
	CreatedAt    time.Time      `json:"createdAt"`
}

type orgMemberDTO struct {
	userDTO
	Role    domain.OrgRole `json:"role"`
	AddedAt time.Time      `json:"addedAt"`
}

type invitationDTO struct {
	ID        int64          `json:"id"`
	Email     string         `json:"email"`
	Role      domain.OrgRole `json:"role"`
	CreatedAt time.Time      `json:"createdAt"`
}

type projectDTO struct {
	ID               int64       `json:"id"`
	OrganizationID   int64       `json:"organizationId"`
	OrganizationName string      `json:"organizationName"`
	Name             string      `json:"name"`
	Key              string      `json:"key"`
	OwnerID          int64       `json:"ownerId"`
	Role             domain.Role `json:"role"`
	MemberCount      int         `json:"memberCount"`
	CardCount        int         `json:"cardCount"`
	CreatedAt        time.Time   `json:"createdAt"`
}

type columnDTO struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	CardCount int    `json:"cardCount"`
}

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

type attachmentDTO struct {
	ID          int64                 `json:"id"`
	CardID      int64                 `json:"cardId"`
	FileName    string                `json:"fileName"`
	ContentType string                `json:"contentType"`
	Size        int64                 `json:"size"`
	Kind        domain.AttachmentKind `json:"kind"`
	UploaderID  *int64                `json:"uploaderId"`
	CreatedAt   time.Time             `json:"createdAt"`
	URL         string                `json:"url"`         // inline view (images, videos)
	DownloadURL string                `json:"downloadUrl"` // always a download
	ThumbURL    *string               `json:"thumbUrl"`
}

type assignedCardDTO struct {
	cardDTO
	ProjectName string `json:"projectName"`
	ColumnName  string `json:"columnName"`
}

type dashboardDTO struct {
	Stats struct {
		Organizations int `json:"organizations"`
		Projects      int `json:"projects"`
		Assigned      int `json:"assigned"`
		Urgent        int `json:"urgent"`
		Created       int `json:"created"`
	} `json:"stats"`
	Assigned []assignedCardDTO `json:"assigned"`
}

// sessionDTO answers login, registration and refresh. The refresh token travels only in its HttpOnly cookie.
type sessionDTO struct {
	User        meDTO     `json:"user"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

func toProject(p domain.Project) projectDTO {
	return projectDTO{p.ID, p.OrgID, p.OrgName, p.Name, p.Key, p.OwnerID, p.Role, p.MemberCount, p.CardCount, p.CreatedAt}
}

func toOrg(o domain.Organization) orgDTO {
	return orgDTO{o.ID, o.Name, o.Role, o.MemberCount, o.ProjectCount, o.CreatedAt}
}

func toInvitation(i domain.Invitation) invitationDTO {
	return invitationDTO{i.ID, i.Email, i.Role, i.CreatedAt}
}

func toColumns(cs []domain.Column) []columnDTO {
	out := make([]columnDTO, len(cs))
	for i, c := range cs {
		out[i] = columnDTO{c.ID, c.ProjectID, c.Name, c.Position, c.CardCount}
	}
	return out
}

func cursorString(c *domain.CardCursor) *string {
	if c == nil {
		return nil
	}
	s := fmt.Sprintf("%d:%d", c.Position, c.ID)
	return &s
}

func mapSlice[T, R any](in []T, f func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
