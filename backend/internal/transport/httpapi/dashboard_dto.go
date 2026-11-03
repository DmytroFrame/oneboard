package httpapi

import (
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/dashboard"
)

// JSON shapes of the dashboard module.

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

func (p *Presenter) Dashboard(d dashboard.Dashboard) dashboardDTO {
	var out dashboardDTO
	out.Stats.Organizations = d.Stats.Organizations
	out.Stats.Projects = d.Stats.Projects
	out.Stats.Assigned = d.Stats.Assigned
	out.Stats.Urgent = d.Stats.Urgent
	out.Stats.Created = d.Stats.Created
	out.Assigned = mapSlice(d.Assigned, func(a domain.AssignedCard) assignedCardDTO {
		return assignedCardDTO{p.Card(a.Card), a.ProjectName, a.ColumnName}
	})
	return out
}

type personDateDTO struct {
	User  userDTO     `json:"user"`
	Date  domain.Date `json:"date"`
	Years int         `json:"years,omitempty"`
}

type companyDTO struct {
	Counters struct {
		Employees   int  `json:"employees"`
		Contractors int  `json:"contractors"`
		MyProjects  int  `json:"myProjects"`
		Candidates  *int `json:"candidates"` // null when hidden
	} `json:"counters"`
	Birthdays     []personDateDTO `json:"birthdays"` // null when hidden for the viewer
	Anniversaries []personDateDTO `json:"anniversaries"`
	OnTrial       []personDateDTO `json:"onTrial"` // null when hidden
}

func (p *Presenter) Company(c dashboard.Company) companyDTO {
	var out companyDTO
	out.Counters.Employees, out.Counters.Contractors, out.Counters.MyProjects = c.Counters.Employees, c.Counters.Contractors, c.Counters.MyProjects
	if c.ShowCandidates {
		n := c.Counters.Candidates
		out.Counters.Candidates = &n
	}
	conv := func(list []domain.PersonDate) []personDateDTO {
		if list == nil {
			return nil
		}
		return mapSlice(list, func(pd domain.PersonDate) personDateDTO { return personDateDTO{p.User(pd.User), pd.Date, pd.Years} })
	}
	out.Birthdays, out.Anniversaries, out.OnTrial = conv(c.Birthdays), conv(c.Anniversaries), conv(c.OnTrial)
	return out
}
