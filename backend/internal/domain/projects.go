package domain

import "time"

// Projects and their members.

type Role string

const (
	RoleNone   Role = ""
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type Project struct {
	ID          int64
	OrgID       int64
	OrgName     string
	Name        string
	Key         string
	OwnerID     int64
	Role        Role // the current user's role
	MemberCount int
	CardCount   int
	CreatedAt   time.Time
}

type Member struct {
	User
	Role          Role
	AssignedCount int // cards assigned to them in the project
}
