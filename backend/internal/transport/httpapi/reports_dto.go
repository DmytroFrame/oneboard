package httpapi

import "github.com/DmytroFrame/oneboard/backend/internal/domain"

type timeSumDTO struct {
	ID              int64        `json:"id"`
	Name            string       `json:"name"`
	Day             *domain.Date `json:"day,omitempty"`
	Minutes         int          `json:"minutes"`
	BillableMinutes int          `json:"billableMinutes"`
	Count           int          `json:"count"`
}

func timeSumDTOOf(s domain.TimeSum) timeSumDTO {
	return timeSumDTO{s.ID, s.Name, s.Day, s.Minutes, s.BillableMinutes, s.Count}
}

type personProjectDTO struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Billable        bool   `json:"billable"`
	Minutes         int    `json:"minutes"`
	BillableMinutes int    `json:"billableMinutes"`
	BenchMinutes    int    `json:"benchMinutes"`
}

// personReportDTO carries the raw sums and the derived values, so the formulas live only here.
type personReportDTO struct {
	User              userDTO                     `json:"user"`
	Role              domain.CompanyRole          `json:"role"`
	NormMinutes       int                         `json:"normMinutes"`
	Absence           map[domain.VacationType]int `json:"absence"`
	Minutes           int                         `json:"minutes"`
	BillableMinutes   int                         `json:"billableMinutes"`
	BenchMinutes      int                         `json:"benchMinutes"`      // tracked on "bench" tasks
	NotBillableByNorm int                         `json:"notBillableByNorm"` // norm − billable − absences (bench %)
	NotTracked        int                         `json:"notTrackedMinutes"`
	OverNorm          int                         `json:"overNormMinutes"`
	Projects          []personProjectDTO          `json:"projects"`
}

func (p *Presenter) PersonReport(r domain.PersonReport) personReportDTO {
	absence := map[domain.VacationType]int{domain.VacationRegular: 0, domain.VacationSick: 0, domain.VacationDayOff: 0}
	for t, m := range r.Absence {
		absence[t] = m
	}
	return personReportDTO{
		User: p.User(r.User), Role: r.User.Role, NormMinutes: r.NormMinutes, Absence: absence,
		Minutes: r.Minutes, BillableMinutes: r.BillableMinutes, BenchMinutes: r.BenchMinutes,
		NotBillableByNorm: r.BenchMinutesByNorm(), NotTracked: r.NotTrackedMinutes(), OverNorm: r.OverNormMinutes(),
		Projects: mapSlice(r.Projects, func(t domain.PersonProjectTime) personProjectDTO {
			return personProjectDTO{t.ProjectID, t.ProjectName, t.ProjectBillable, t.Minutes, t.BillableMinutes, t.BenchMinutes}
		}),
	}
}

type statsBucketDTO struct {
	Start      domain.Date `json:"start"`
	Created    int         `json:"created"`
	Done       int         `json:"done"`
	Stays      int         `json:"stays"`
	AvgMinutes int         `json:"avgMinutes"`
}

type taskStatsDTO struct {
	Buckets    []statsBucketDTO `json:"buckets"`
	Stays      int              `json:"stays"`
	AvgMinutes int              `json:"avgMinutes"`
}

func taskStatsDTOOf(s domain.TaskStats) taskStatsDTO {
	return taskStatsDTO{
		Buckets: mapSlice(s.Buckets, func(b domain.StatsBucket) statsBucketDTO {
			return statsBucketDTO{b.Start, b.Created, b.Done, b.Stays, b.AvgMinutes}
		}),
		Stays: s.Stays, AvgMinutes: s.AvgMinutes,
	}
}
