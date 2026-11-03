package domain

import (
	"strings"
	"time"
)

// Projects and their members.

type Role string

const (
	RoleNone   Role = ""
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin" // project admin (OmiTeam): logs time for others and deletes tasks, with LEAD
	RoleMember Role = "member"
)

type Project struct {
	ID          int64
	OrgID       int64
	OrgName     string
	Name        string
	Key         string
	OwnerID     int64
	Role        Role // the current user's role ("" when they see it through viewAllProjects)
	MemberCount int
	CardCount   int
	CreatedAt   time.Time
	ProjectSettings
	Pinned bool // by the current user
}

// ProjectSettings are the OmiTeam project fields.
type ProjectSettings struct {
	Client      string
	Description string
	StartDate   *Date
	Active      bool
	Billable    bool     // counts in billability and bench reports
	BudgetHours *float64 // nil = no budget
	IsMain      bool     // the company's own project, where special tasks (vacation, bench…) belong
	Sections    ProjectSections
}

// ProjectSections turns project tabs on and off.
type ProjectSections struct {
	Backlog    bool `json:"backlog"`
	Planning   bool `json:"planning"` // milestones, sprints, versions
	Comments   bool `json:"comments"`
	Statistics bool `json:"statistics"`
	Wiki       bool `json:"wiki"`
}

func (s *ProjectSettings) Normalize() error {
	s.Client, s.Description = strings.TrimSpace(s.Client), strings.TrimSpace(s.Description)
	if len(s.Client) > 100 || len(s.Description) > 5000 {
		return ErrProjectSettings
	}
	if s.BudgetHours != nil && *s.BudgetHours <= 0 {
		return ErrProjectSettings
	}
	return nil
}

type Member struct {
	User
	Role          Role
	AssignedCount int // cards assigned to them in the project
}
