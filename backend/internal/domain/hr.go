package domain

import (
	"slices"
	"strings"
	"time"
)

// CandidateStatus is a step of the hiring funnel (and a few side tracks).
type CandidateStatus string

const (
	CandidateNew             CandidateStatus = "new"
	CandidateHRInterview     CandidateStatus = "hr_interview"
	CandidateEnglishCheck    CandidateStatus = "english_check"
	CandidateClientInterview CandidateStatus = "client_interview"
	CandidateOnboarding      CandidateStatus = "onboarding"
	CandidateInTeam          CandidateStatus = "in_team"
	CandidateBlacklist       CandidateStatus = "blacklist"
	CandidatePingAgain       CandidateStatus = "ping_again"
	CandidateContractor      CandidateStatus = "contractor"
)

var CandidateStatuses = []CandidateStatus{CandidateNew, CandidateHRInterview, CandidateEnglishCheck, CandidateClientInterview,
	CandidateOnboarding, CandidateInTeam, CandidateBlacklist, CandidatePingAgain, CandidateContractor}

func (s CandidateStatus) Valid() bool { return slices.Contains(CandidateStatuses, s) }

// CandidateAttributes are the labels a candidate can carry.
var CandidateAttributes = []string{"hr_interview", "english_check", "ping_again", "test_task_done", "part_time", "full_time", "promising"}

// RejectionReasons is OmiTeam's fixed list of reasons for turning a candidate down.
var RejectionReasons = []string{"no_commercial_experience", "low_english", "salary_too_high", "stopped_responding",
	"wants_office", "failed_interview", "declined_offer", "toxic_behaviour", "overestimates_skills"}

// Candidate is a CANDIDATE account with its recruiting data; the contacts and links are in its profile.
type Candidate struct {
	User               User
	Specialization     string
	Grade              string
	Status             CandidateStatus
	Attributes         []string
	Channel            string
	MinSalary          *float64
	MaxSalary          *float64
	MinRate            *float64
	MaxRate            *float64
	HiringCommission   *float64
	CommissionReceiver string
	TrelloLink         string
	Recruiter          *User
	RejectionReason    string
}

// Normalize checks a candidate's recruiting data.
func (c *Candidate) Normalize() error {
	c.Channel, c.CommissionReceiver, c.TrelloLink = strings.TrimSpace(c.Channel), strings.TrimSpace(c.CommissionReceiver), strings.TrimSpace(c.TrelloLink)
	if !c.Status.Valid() || len(c.Channel) > 100 || len(c.CommissionReceiver) > 200 || len(c.TrelloLink) > 500 {
		return ErrCandidateInvalid
	}
	seen := map[string]bool{}
	attrs := []string{}
	for _, a := range c.Attributes {
		if !slices.Contains(CandidateAttributes, a) {
			return ErrCandidateInvalid
		}
		if !seen[a] {
			seen[a] = true
			attrs = append(attrs, a)
		}
	}
	c.Attributes = attrs
	for _, pair := range [][2]*float64{{c.MinSalary, c.MaxSalary}, {c.MinRate, c.MaxRate}} {
		for _, v := range pair {
			if v != nil && *v < 0 {
				return ErrCandidateInvalid
			}
		}
		if pair[0] != nil && pair[1] != nil && *pair[0] > *pair[1] {
			return ErrCandidateInvalid
		}
	}
	if c.HiringCommission != nil && *c.HiringCommission < 0 {
		return ErrCandidateInvalid
	}
	return nil
}

// RedactMoney hides salary expectations and the commission from people who can't see salaries.
func (c *Candidate) RedactMoney() {
	c.MinSalary, c.MaxSalary, c.MinRate, c.MaxRate, c.HiringCommission, c.CommissionReceiver = nil, nil, nil, nil, nil, ""
}

type CandidateFilter struct {
	Q              string
	Status         CandidateStatus
	Attribute      string
	Specialization string
	RecruiterID    int64
	Offset, Limit  int
}

type CandidatePage struct {
	Candidates []Candidate
	Total      int
}

// HREventKind: what the HR statistics count.
type HREventKind string

const (
	HRHired     HREventKind = "hired"
	HRDismissed HREventKind = "dismissed"
	HRRejected  HREventKind = "rejected" // a candidate closed unsuccessfully
)

type HREvent struct {
	ID            int64
	OrgID         int64
	User          User
	Kind          HREventKind
	Day           Date
	FromCandidate bool // a hire that closed a candidate successfully
	Reason        string
	ActorID       *int64
	CreatedAt     time.Time
}

// HRMonth is one month of the hires-and-dismissals chart.
type HRMonth struct {
	Month     int
	Hired     int
	Dismissed int
}

type HRStats struct {
	Months        []HRMonth
	ClosedSuccess int // candidates hired
	ClosedFail    int // candidates rejected
	Hired         []HREvent
	Dismissed     []HREvent
}
