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
