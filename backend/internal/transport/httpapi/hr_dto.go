package httpapi

import "github.com/DmytroFrame/oneboard/backend/internal/domain"

type candidateDTO struct {
	User               userDTO                `json:"user"`
	Active             bool                   `json:"active"`
	Specialization     string                 `json:"specialization"`
	Grade              string                 `json:"grade"`
	Status             domain.CandidateStatus `json:"status"`
	Attributes         []string               `json:"attributes"`
	Channel            string                 `json:"channel"`
	MinSalary          *float64               `json:"minSalary"`
	MaxSalary          *float64               `json:"maxSalary"`
	MinRate            *float64               `json:"minRate"`
	MaxRate            *float64               `json:"maxRate"`
	HiringCommission   *float64               `json:"hiringCommission"`
	CommissionReceiver string                 `json:"commissionReceiver"`
	TrelloLink         string                 `json:"trelloLink"`
	Recruiter          *userDTO               `json:"recruiter"`
	RejectionReason    string                 `json:"rejectionReason"`
}

func (p *Presenter) Candidate(c domain.Candidate) candidateDTO {
	d := candidateDTO{
		User: p.User(c.User), Active: c.User.Active, Specialization: c.Specialization, Grade: c.Grade, Status: c.Status,
		Attributes: nonNil(c.Attributes), Channel: c.Channel, MinSalary: c.MinSalary, MaxSalary: c.MaxSalary,
		MinRate: c.MinRate, MaxRate: c.MaxRate, HiringCommission: c.HiringCommission, CommissionReceiver: c.CommissionReceiver,
		TrelloLink: c.TrelloLink, RejectionReason: c.RejectionReason,
	}
	if c.Recruiter != nil {
		r := p.User(*c.Recruiter)
		d.Recruiter = &r
	}
	return d
}

// candidateInput is the candidate form; contacts and links are edited on the profile.
type candidateInput struct {
	Status             domain.CandidateStatus `json:"status"`
	Attributes         []string               `json:"attributes"`
	Channel            string                 `json:"channel"`
	MinSalary          *float64               `json:"minSalary"`
	MaxSalary          *float64               `json:"maxSalary"`
	MinRate            *float64               `json:"minRate"`
	MaxRate            *float64               `json:"maxRate"`
	HiringCommission   *float64               `json:"hiringCommission"`
	CommissionReceiver string                 `json:"commissionReceiver"`
	TrelloLink         string                 `json:"trelloLink"`
	RecruiterID        *int64                 `json:"recruiterId"`
}

func (in candidateInput) domain() domain.Candidate {
	c := domain.Candidate{
		Status: in.Status, Attributes: in.Attributes, Channel: in.Channel, MinSalary: in.MinSalary, MaxSalary: in.MaxSalary,
		MinRate: in.MinRate, MaxRate: in.MaxRate, HiringCommission: in.HiringCommission, CommissionReceiver: in.CommissionReceiver,
		TrelloLink: in.TrelloLink,
	}
	if in.RecruiterID != nil {
		c.Recruiter = &domain.User{ID: *in.RecruiterID}
	}
	return c
}

type hrEventDTO struct {
	ID            int64              `json:"id"`
	User          userDTO            `json:"user"`
	Role          domain.CompanyRole `json:"role"`
	Kind          domain.HREventKind `json:"kind"`
	Day           domain.Date        `json:"day"`
	FromCandidate bool               `json:"fromCandidate"`
	Reason        string             `json:"reason"`
}

type hrMonthDTO struct {
	Month     int `json:"month"`
	Hired     int `json:"hired"`
	Dismissed int `json:"dismissed"`
}

type hrStatsDTO struct {
	Months        []hrMonthDTO `json:"months"`
	ClosedSuccess int          `json:"closedSuccess"`
	ClosedFail    int          `json:"closedFail"`
	Hired         []hrEventDTO `json:"hired"`
	Dismissed     []hrEventDTO `json:"dismissed"`
}

func (p *Presenter) HRStats(s domain.HRStats) hrStatsDTO {
	event := func(e domain.HREvent) hrEventDTO {
		return hrEventDTO{e.ID, p.User(e.User), e.User.Role, e.Kind, e.Day, e.FromCandidate, e.Reason}
	}
	return hrStatsDTO{
		Months:        mapSlice(s.Months, func(m domain.HRMonth) hrMonthDTO { return hrMonthDTO{m.Month, m.Hired, m.Dismissed} }),
		ClosedSuccess: s.ClosedSuccess, ClosedFail: s.ClosedFail,
		Hired: mapSlice(s.Hired, event), Dismissed: mapSlice(s.Dismissed, event),
	}
}
