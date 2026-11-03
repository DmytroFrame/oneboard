package domain

import (
	"slices"
	"strings"
	"time"
)

type Organization struct {
	ID           int64
	Name         string
	Role         CompanyRole // the current user's role there
	MemberCount  int
	ProjectCount int
	CreatedAt    time.Time
}

// Managing an organization uses the company roles of its members (company.go): owners and admins
// rename it, owners delete it; inviting, removing and changing roles follow the permission matrix.
func (u User) ManagesOrg() bool { return u.Role == CompanyOwner || u.Role == CompanyAdmin }

// CanInviteAs reports whether u may invite someone with the given role (candidates included).
func (u User) CanInviteAs(role CompanyRole) bool {
	if !u.Can(PermAddUser) {
		return false
	}
	return role == CompanyCandidate || slices.Contains(u.AssignableRoles(), role)
}

func NormalizeOrgName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return "", ErrOrgName
	}
	return name, nil
}

// OrgMember is a person with their membership filled in (User.OrgID, Role…).
type OrgMember struct {
	User
	AddedAt time.Time
}

type Invitation struct {
	ID        int64
	OrgID     int64
	Email     string
	Role      CompanyRole
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
