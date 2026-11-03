package domain

import (
	"strings"
	"time"
)

// --- time tracking ---

// MinuteStep: time is logged in 5-minute steps.
const MinuteStep = 5

type Timelog struct {
	ID        int64
	UserID    int64
	CardID    int64
	ProjectID int64
	Day       Date
	Minutes   int
	Comment   string
	CreatedBy *int64
	CreatedAt time.Time

	// For lists: what the time was logged on.
	UserName    string
	CardKey     string
	CardTitle   string
	ProjectName string
	Billable    bool // the project and the task are both billable
}

// TimelogInput is a new or changed entry.
type TimelogInput struct {
	UserID  int64 // 0 = the actor
	CardID  int64
	Day     Date
	Minutes int
	Comment string
}

func (in *TimelogInput) Validate() error {
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Minutes <= 0 {
		return ErrTimeZero
	}
	if in.Minutes > 24*60 || in.Minutes%MinuteStep != 0 || len(in.Comment) > 2000 || in.Day.IsZero() {
		return ErrTimelogInvalid
	}
	return nil
}

// DayStats is one day of a week's statistics.
type DayStats struct {
	Day             Date
	Minutes         int
	BillableMinutes int
	NormMinutes     int // what the day should have (0 on weekends, holidays and approved absences)
	Holiday         string
	Absence         VacationType // "" or the approved absence covering the day
}

// WeekStats: Monday to Sunday.
type WeekStats struct {
	Days            []DayStats
	Minutes         int
	BillableMinutes int
	NormMinutes     int
	OvertimeMinutes int // above the norm, including any weekend or holiday work
}

// MondayOf returns the Monday of d's week.
func MondayOf(d Date) Date {
	wd := int(d.Weekday())
	if wd == 0 {
		wd = 7
	}
	return d.AddDays(1 - wd)
}

// --- holidays & vacations ---

type Holiday struct {
	ID           int64
	Day          Date
	NameLatin    string
	NameCyrillic string
}

type VacationType string

const (
	VacationRegular VacationType = "vacation"
	VacationSick    VacationType = "sick_day"
	VacationDayOff  VacationType = "day_off" // unpaid
)

func (t VacationType) Valid() bool {
	return t == VacationRegular || t == VacationSick || t == VacationDayOff
}

type VacationStatus string

const (
	VacationPending  VacationStatus = "pending"
	VacationApproved VacationStatus = "approved"
	VacationDeclined VacationStatus = "declined"
)

// Yearly allowances in working days (decision recorded in docs/omiteam-integration.md).
var VacationAllowance = map[VacationType]int{VacationRegular: 24, VacationSick: 10, VacationDayOff: 10}

type Vacation struct {
	ID          int64
	UserID      int64
	UserName    string
	Type        VacationType
	Start, End  Date
	Days        int // working days, without weekends and holidays
	Comment     string
	Status      VacationStatus
	DecidedBy   *int64
	DecidedAt   *time.Time
	CreatedAt   time.Time
	ReviewerIDs []int64
}

// VacationFilter for the "all requests" list.
type VacationFilter struct {
	UserID        int64 // 0 = everyone
	Year          int
	Month         int // 0 = whole year
	Status        VacationStatus
	Type          VacationType
	RequesterName string
}

// VacationStats: days taken and left, per type, in a year.
type VacationStats struct {
	Year  int
	Taken map[VacationType]int
	Left  map[VacationType]int
}

// WorkingDays counts Monday–Friday days between start and end (inclusive) that are not holidays.
func WorkingDays(start, end Date, holidays map[string]bool) int {
	n := 0
	for d := start; !d.After(end.Time); d = d.AddDays(1) {
		if !d.Weekend() && !holidays[d.String()] {
			n++
		}
	}
	return n
}
