package domain

import (
	"slices"
	"strings"
	"time"
)

// Grades and English levels (OmiTeam's reference values).
var (
	Grades        = []string{"Trainee", "Junior", "Strong Junior", "Middle", "Strong Middle", "Senior"}
	EnglishLevels = []string{"Beginner", "Elementary", "Pre-intermediate", "Intermediate", "Upper-Intermediate", "Advanced", "Proficiency"}
	// EmergencyRelations: who an emergency contact is to the person.
	EmergencyRelations = []string{"mother", "father", "wife", "husband", "boyfriend", "girlfriend", "friend", "other"}
)

// Profile is everything about a person beyond their account. Which parts a viewer gets is decided by
// ProfileAccess.
type Profile struct {
	User         User
	NameCyrillic string
	Birthday     *Date
	StartDate    *Date
	TrialUntil   *Date // on probation until (inclusive)
	WorkingHours int   // per working day, 0–24
	Billable     bool
	Details      ProfileDetails
	Finance      ProfileFinance
}

// ProfileDetails are free-form fields, stored as one JSON document.
type ProfileDetails struct {
	About          string           `json:"about,omitempty"`
	Phone          string           `json:"phone,omitempty"`
	Telegram       string           `json:"telegram,omitempty"`
	WhatsApp       string           `json:"whatsapp,omitempty"`
	CompanyEmail   string           `json:"companyEmail,omitempty"`
	Country        string           `json:"country,omitempty"`
	City           string           `json:"city,omitempty"`
	PostAddress    string           `json:"postAddress,omitempty"` // Nova Poshta branch
	Specialization string           `json:"specialization,omitempty"`
	TechStack      []string         `json:"techStack,omitempty"`
	Grade          string           `json:"grade,omitempty"`
	EnglishLevel   string           `json:"englishLevel,omitempty"`
	GitHub         string           `json:"github,omitempty"`
	GitLab         string           `json:"gitlab,omitempty"`
	LinkedIn       string           `json:"linkedin,omitempty"`
	Upwork         string           `json:"upwork,omitempty"`
	CVLink         string           `json:"cvLink,omitempty"`
	OtherLinks     string           `json:"otherLinks,omitempty"`
	Emergency      EmergencyContact `json:"emergency,omitzero"`
	HRNotes        string           `json:"hrNotes,omitempty"`
}

type EmergencyContact struct {
	Name     string `json:"name,omitempty"`
	Relation string `json:"relation,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Telegram string `json:"telegram,omitempty"`
}

// ProfileFinance: salary and payment details.
type ProfileFinance struct {
	Salary         float64 `json:"salary,omitempty"`
	Rate           float64 `json:"rate,omitempty"` // hourly
	Currency       string  `json:"currency,omitempty"`
	FOP            bool    `json:"fop,omitempty"` // registered sole proprietor
	PayoneerEmail  string  `json:"payoneerEmail,omitempty"`
	MonobankCard   string  `json:"monobankCard,omitempty"`
	PrivatBankCard string  `json:"privatBankCard,omitempty"`
	TaxID          string  `json:"taxId,omitempty"` // IPN
}

// Normalize trims text fields and validates reference values.
func (p *Profile) Normalize() error {
	p.NameCyrillic = strings.TrimSpace(p.NameCyrillic)
	if p.WorkingHours < 0 || p.WorkingHours > 24 {
		return ErrWorkingHours
	}
	d := &p.Details
	if d.Grade != "" && !slices.Contains(Grades, d.Grade) {
		return ErrProfileValue
	}
	if d.EnglishLevel != "" && !slices.Contains(EnglishLevels, d.EnglishLevel) {
		return ErrProfileValue
	}
	if d.Emergency.Relation != "" && !slices.Contains(EmergencyRelations, d.Emergency.Relation) {
		return ErrProfileValue
	}
	if len(d.About) > 5000 || len(d.HRNotes) > 5000 || len(d.TechStack) > 30 {
		return ErrProfileValue
	}
	if p.Finance.Salary < 0 || p.Finance.Rate < 0 {
		return ErrProfileValue
	}
	return nil
}

// ProfileAccess is what a viewer may see and change of someone's profile.
type ProfileAccess struct {
	Self        bool
	SeeContacts bool // phone, messengers, address, birthday
	SeeGrade    bool
	SeeHR       bool // emergency contact, HR notes, probation
	SeeFinance  bool
	EditBasics  bool // about, contacts, links, skills: the person themself or HR
	EditWork    bool // start date, hours, billable, probation, grade, specialization: HR only
	EditFinance bool
}

// AccessTo decides what viewer may do with target's profile.
func AccessTo(viewer, target User) ProfileAccess {
	self := viewer.ID == target.ID
	hr := viewer.Can(PermEditUser)
	return ProfileAccess{
		Self:        self,
		SeeContacts: self || viewer.Can(PermViewCrewBlock),
		SeeGrade:    self || viewer.Can(PermViewGrades),
		SeeHR:       self || viewer.Can(PermSeeHRInformation),
		SeeFinance:  self || viewer.Can(PermViewSalary),
		EditBasics:  hr || (self && viewer.Can(PermEditProfile)),
		EditWork:    hr,
		EditFinance: viewer.Can(PermViewSalary) && hr,
	}
}

// Redact clears what the viewer may not see.
func (a ProfileAccess) Redact(p Profile) Profile {
	if !a.SeeContacts {
		p.Birthday = nil
		d := p.Details
		p.Details = ProfileDetails{About: d.About, Specialization: d.Specialization, TechStack: d.TechStack, EnglishLevel: d.EnglishLevel,
			GitHub: d.GitHub, GitLab: d.GitLab, LinkedIn: d.LinkedIn, CompanyEmail: d.CompanyEmail, Grade: d.Grade}
	}
	if !a.SeeGrade {
		p.Details.Grade = ""
	}
	if !a.SeeHR {
		p.TrialUntil = nil
		p.Details.Emergency = EmergencyContact{}
		p.Details.HRNotes = ""
		p.Details.PostAddress = ""
	}
	if !a.SeeFinance {
		p.Finance = ProfileFinance{}
	}
	return p
}

// Date is a calendar day without a time zone ("2026-11-02").
type Date struct{ time.Time }

func NewDate(y int, m time.Month, d int) Date { return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)} }

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, ErrDateInvalid
	}
	return Date{t}, nil
}

func (d Date) String() string { return d.Format(time.DateOnly) }

// Today in the server's time zone, as a date.
func Today() Date { return DateOf(time.Now()) }

// DateOf is the calendar date of a moment in the server's time zone.
func DateOf(t time.Time) Date {
	y, m, dd := t.Local().Date()
	return NewDate(y, m, dd)
}

func (d Date) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// AddDays returns the date n days later (or earlier).
func (d Date) AddDays(n int) Date { return Date{d.Time.AddDate(0, 0, n)} }

// Weekend reports Saturday or Sunday.
func (d Date) Weekend() bool { return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday }

// --- people directory ---

// PersonFilter narrows the team list.
type PersonFilter struct {
	Query           string        // name or email
	Quick           string        // "", "billable", "nonbillable", "contractors"
	Roles           []CompanyRole // empty: everyone except candidates
	Specialization  string
	TechStack       string
	Grade           string
	EnglishLevel    string
	City            string
	IncludeInactive bool
	Sort            string // "name", "role", "startDate"
	Desc            bool
	Offset, Limit   int
}

type PersonPage struct {
	People []Profile
	Total  int
}

// DirectoryKind names an editable reference list.
type DirectoryKind string

const (
	DirSpecialization   DirectoryKind = "specialization"
	DirTechStack        DirectoryKind = "tech_stack"
	DirCandidateChannel DirectoryKind = "candidate_channel"
)

func (k DirectoryKind) Valid() bool {
	return k == DirSpecialization || k == DirTechStack || k == DirCandidateChannel
}

type DirectoryItem struct {
	ID   int64
	Kind DirectoryKind
	Name string
}

// --- comments, feedback, documents ---

// CommentKinds of internal notes about a person.
var CommentKinds = []string{"candidate_feedback", "english_check", "test_task", "interview_feedback", "rejection_reason", "dismissal", "other"}

type PersonComment struct {
	ID         int64
	UserID     int64
	AuthorID   *int64 // nil when anonymous (hidden from everyone) or the author is gone
	AuthorName string
	Kind       string
	Body       string
	Anonymous  bool
	CreatedAt  time.Time
}

type Feedback struct {
	ID         int64
	UserID     int64
	AuthorID   *int64
	AuthorName string
	StartDoing string
	StopDoing  string
	KeepDoing  string
	Anonymous  bool
	CreatedAt  time.Time
}

// DocumentTypes of a person's documents and links.
var DocumentTypes = []string{"contract", "nda", "cv", "certificate", "id", "other"}

const MaxDocumentBytes = 20 << 20

type Document struct {
	ID          int64
	UserID      int64
	UploaderID  *int64
	Name        string
	Description string
	Type        string
	Private     bool
	Link        string
	StorageKey  string
	FileName    string
	ContentType string
	Size        int64
	CreatedAt   time.Time
}

// --- dashboard (company widgets) ---

// CompanyCounters: the dashboard's headline numbers.
type CompanyCounters struct {
	Employees   int // active people other than contractors and candidates
	Contractors int
	MyProjects  int
	Candidates  int
}

// PersonDate is someone with a date of interest (birthday, work anniversary, end of probation).
type PersonDate struct {
	User  User
	Date  Date // the next occurrence
	Years int  // anniversaries: years in the company on that date
}

// DayWork is one day of the work-time chart.
type DayWork struct {
	Day             Date
	BillableMinutes int
	OtherMinutes    int
}
