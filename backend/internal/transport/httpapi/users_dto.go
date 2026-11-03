package httpapi

import (
	"fmt"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the users module.

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
	// The company role and what it allows, so the UI doesn't need its own copy of the matrix to decide.
	Role          domain.CompanyRole    `json:"role"`
	AdditionRoles []domain.AdditionRole `json:"additionRoles"`
	Permissions   []domain.Permission   `json:"permissions"`
	// AssignableRoles: roles this user may give to others (empty = can't change roles).
	AssignableRoles []domain.CompanyRole `json:"assignableRoles"`
	// The organization the role is in (null: none yet), and every organization for the switcher.
	Organization  *membershipDTO  `json:"organization"`
	Organizations []membershipDTO `json:"organizations"`
	PlatformAdmin bool            `json:"platformAdmin"`
}

type membershipDTO struct {
	ID     int64              `json:"id"`
	Name   string             `json:"name"`
	Role   domain.CompanyRole `json:"role"`
	Active bool               `json:"active"`
}

// Avatars have permanent URLs, versioned by upload time.
func avatarMessage(userID, version int64) string { return fmt.Sprintf("avatar:%d:%d", userID, version) }

func (p *Presenter) validAvatarSig(userID, version int64, sig string) bool {
	return p.verify(avatarMessage(userID, version), sig)
}

func (p *Presenter) User(u domain.User) userDTO {
	d := userDTO{ID: u.ID, Name: u.Name, Email: u.Email, Color: u.Color}
	if u.HasAvatar() {
		var v int64
		if u.AvatarUpdatedAt != nil {
			v = u.AvatarUpdatedAt.UnixMilli()
		}
		url := fmt.Sprintf("/api/users/%d/avatar?v=%d&sig=%s", u.ID, v, p.sign(avatarMessage(u.ID, v)))
		d.AvatarURL = &url
	}
	return d
}

// Me is the signed-in user in the organization the request works in, with their organizations.
func (p *Presenter) Me(u domain.User, ms []domain.Membership) meDTO {
	granted := []domain.Permission{}
	for _, perm := range domain.Permissions() {
		if u.Can(perm) {
			granted = append(granted, perm)
		}
	}
	d := meDTO{
		userDTO:         p.User(u),
		Settings:        settingsDTO{u.Settings.Language, u.Settings.Theme},
		Role:            u.Role,
		AdditionRoles:   nonNil(u.AdditionRoles),
		Permissions:     granted,
		AssignableRoles: nonNil(u.AssignableRoles()),
		Organizations:   []membershipDTO{},
		PlatformAdmin:   u.PlatformAdmin,
	}
	for _, m := range ms {
		dto := membershipDTO{m.OrgID, m.OrgName, m.Role, m.Active}
		d.Organizations = append(d.Organizations, dto)
		if m.OrgID == u.OrgID {
			d.Organization = &dto
		}
	}
	return d
}
