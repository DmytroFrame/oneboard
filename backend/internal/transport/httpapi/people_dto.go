package httpapi

import (
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the people module (team directory and profiles).

type accessDTO struct {
	Self        bool `json:"self"`
	SeeContacts bool `json:"seeContacts"`
	SeeGrade    bool `json:"seeGrade"`
	SeeHR       bool `json:"seeHR"`
	SeeFinance  bool `json:"seeFinance"`
	EditBasics  bool `json:"editBasics"`
	EditWork    bool `json:"editWork"`
	EditFinance bool `json:"editFinance"`
}

type profileDTO struct {
	userDTO
	Role            domain.CompanyRole     `json:"role"`
	AdditionRoles   []domain.AdditionRole  `json:"additionRoles"`
	Active          bool                   `json:"active"`
	DismissedAt     *time.Time             `json:"dismissedAt"`
	DismissalReason string                 `json:"dismissalReason"`
	NameCyrillic    string                 `json:"nameCyrillic"`
	Birthday        *domain.Date           `json:"birthday"`
	StartDate       *domain.Date           `json:"startDate"`
	TrialUntil      *domain.Date           `json:"trialUntil"`
	WorkingHours    int                    `json:"workingHours"`
	Billable        bool                   `json:"billable"`
	Details         domain.ProfileDetails  `json:"details"`
	Finance         *domain.ProfileFinance `json:"finance"` // null when hidden
	Access          accessDTO              `json:"access"`
}

// profileInput is the profile form; fields the editor may not change are ignored by the use case.
type profileInput struct {
	Name         string                `json:"name"`
	NameCyrillic string                `json:"nameCyrillic"`
	Birthday     *domain.Date          `json:"birthday"`
	StartDate    *domain.Date          `json:"startDate"`
	TrialUntil   *domain.Date          `json:"trialUntil"`
	WorkingHours int                   `json:"workingHours"`
	Billable     bool                  `json:"billable"`
	Details      domain.ProfileDetails `json:"details"`
	Finance      domain.ProfileFinance `json:"finance"`
}

type personPageDTO struct {
	People []profileDTO `json:"people"`
	Total  int          `json:"total"`
}

func (p *Presenter) Profile(pr domain.Profile, a domain.ProfileAccess, viewer domain.User) profileDTO {
	d := profileDTO{
		userDTO: p.User(pr.User), Role: pr.User.Role, AdditionRoles: nonNil(pr.User.AdditionRoles), Active: pr.User.Active,
		NameCyrillic: pr.NameCyrillic, Birthday: pr.Birthday, StartDate: pr.StartDate, TrialUntil: pr.TrialUntil,
		WorkingHours: pr.WorkingHours, Billable: pr.Billable, Details: pr.Details,
		Access: accessDTO(a),
	}
	if d.Details.TechStack == nil {
		d.Details.TechStack = []string{}
	}
	if a.SeeFinance {
		f := pr.Finance
		d.Finance = &f
	}
	// Why someone left is for HR (and the person).
	if a.Self || viewer.Can(domain.PermEditUser) {
		d.DismissedAt, d.DismissalReason = pr.User.DismissedAt, pr.User.DismissalReason
	}
	return d
}

type personCommentDTO struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"userId"`
	AuthorID   *int64    `json:"authorId"` // null for anonymous comments
	AuthorName string    `json:"authorName"`
	Kind       string    `json:"kind"`
	Body       string    `json:"body"`
	Anonymous  bool      `json:"anonymous"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toPersonComment(c domain.PersonComment) personCommentDTO {
	return personCommentDTO{c.ID, c.UserID, c.AuthorID, c.AuthorName, c.Kind, c.Body, c.Anonymous, c.CreatedAt}
}

type feedbackDTO struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"userId"`
	AuthorID   *int64    `json:"authorId"`
	AuthorName string    `json:"authorName"`
	StartDoing string    `json:"startDoing"`
	StopDoing  string    `json:"stopDoing"`
	KeepDoing  string    `json:"keepDoing"`
	Anonymous  bool      `json:"anonymous"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toFeedback(f domain.Feedback) feedbackDTO {
	return feedbackDTO{f.ID, f.UserID, f.AuthorID, f.AuthorName, f.StartDoing, f.StopDoing, f.KeepDoing, f.Anonymous, f.CreatedAt}
}

type documentDTO struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"userId"`
	UploaderID  *int64    `json:"uploaderId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Private     bool      `json:"private"`
	Link        string    `json:"link"`
	FileName    string    `json:"fileName"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toDocument(d domain.Document) documentDTO {
	return documentDTO{d.ID, d.UserID, d.UploaderID, d.Name, d.Description, d.Type, d.Private, d.Link, d.FileName, d.Size, d.CreatedAt}
}

type directoryItemDTO struct {
	ID   int64                `json:"id"`
	Kind domain.DirectoryKind `json:"kind"`
	Name string               `json:"name"`
}

func toDirectoryItem(it domain.DirectoryItem) directoryItemDTO {
	return directoryItemDTO{it.ID, it.Kind, it.Name}
}
