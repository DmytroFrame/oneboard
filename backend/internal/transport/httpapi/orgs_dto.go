package httpapi

import (
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the organizations module.

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

func toOrg(o domain.Organization) orgDTO {
	return orgDTO{o.ID, o.Name, o.Role, o.MemberCount, o.ProjectCount, o.CreatedAt}
}

func toInvitation(i domain.Invitation) invitationDTO {
	return invitationDTO{i.ID, i.Email, i.Role, i.CreatedAt}
}

func (p *Presenter) OrgMembers(ms []domain.OrgMember) []orgMemberDTO {
	return mapSlice(ms, func(m domain.OrgMember) orgMemberDTO { return orgMemberDTO{p.User(m.User), m.Role, m.AddedAt} })
}
