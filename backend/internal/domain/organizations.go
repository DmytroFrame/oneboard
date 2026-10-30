package domain

import (
	"strings"
	"time"
)

type Organization struct {
	ID           int64
	Name         string
	Role         OrgRole // the current user's role
	MemberCount  int
	ProjectCount int
	CreatedAt    time.Time
}

type OrgRole string

const (
	OrgRoleNone   OrgRole = ""
	OrgRoleOwner  OrgRole = "owner"
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
)

func (r OrgRole) Valid() bool {
	return r == OrgRoleOwner || r == OrgRoleAdmin || r == OrgRoleMember
}

// The organization permission matrix:
//
//	                 owner  admin  member
//	invite people      ✓      ✓ (admin/member roles only)
//	remove people      ✓      ✓ (members only)
//	change roles       ✓
//	create projects    ✓      ✓
//	rename             ✓      ✓
//	delete             ✓
func (r OrgRole) CanInvite() bool         { return r == OrgRoleOwner || r == OrgRoleAdmin }
func (r OrgRole) CanManageRoles() bool    { return r == OrgRoleOwner }
func (r OrgRole) CanCreateProjects() bool { return r == OrgRoleOwner || r == OrgRoleAdmin }
func (r OrgRole) CanRename() bool         { return r == OrgRoleOwner || r == OrgRoleAdmin }
func (r OrgRole) CanDelete() bool         { return r == OrgRoleOwner }

// CanGrant reports whether someone with role r may hand out role target (on invite).
func (r OrgRole) CanGrant(target OrgRole) bool {
	switch r {
	case OrgRoleOwner:
		return target.Valid()
	case OrgRoleAdmin:
		return target == OrgRoleAdmin || target == OrgRoleMember
	}
	return false
}

// CanRemove reports whether someone with role r may remove a member with role target.
func (r OrgRole) CanRemove(target OrgRole) bool {
	switch r {
	case OrgRoleOwner:
		return true
	case OrgRoleAdmin:
		return target == OrgRoleMember
	}
	return false
}

func NormalizeOrgName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return "", ErrOrgName
	}
	return name, nil
}

type OrgMember struct {
	User
	Role    OrgRole
	AddedAt time.Time
}

type Invitation struct {
	ID        int64
	OrgID     int64
	Email     string
	Role      OrgRole
	InvitedBy *int64
	CreatedAt time.Time
}

// DashboardStats is the signed-in user's overview on the home page.
type DashboardStats struct {
	Organizations int
	Projects      int
	Assigned      int // cards assigned to the user
	Urgent        int // ...of which high or critical priority
	Created       int // cards the user authored
}

// AssignedCard is a card with the context needed to show it outside its board.
type AssignedCard struct {
	Card
	ProjectName string
	ColumnName  string
}
