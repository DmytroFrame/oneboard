package httpapi

import (
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the projects module.

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

type memberDTO struct {
	userDTO
	Role          domain.Role `json:"role"`
	AssignedCount int         `json:"assignedCount"`
}

func toProject(p domain.Project) projectDTO {
	return projectDTO{p.ID, p.OrgID, p.OrgName, p.Name, p.Key, p.OwnerID, p.Role, p.MemberCount, p.CardCount, p.CreatedAt}
}

func (p *Presenter) Members(ms []domain.Member) []memberDTO {
	return mapSlice(ms, func(m domain.Member) memberDTO { return memberDTO{p.User(m.User), m.Role, m.AssignedCount} })
}
