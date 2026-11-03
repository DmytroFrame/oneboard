package domain

import "time"

// MaxReportDays caps a report's period (about three months).
const MaxReportDays = 93

// ReportGroup is what the time report sums by.
type ReportGroup string

const (
	GroupProject   ReportGroup = "project"
	GroupUser      ReportGroup = "user"
	GroupMilestone ReportGroup = "milestone"
	GroupTask      ReportGroup = "task"
	GroupDay       ReportGroup = "day"
)

func (g ReportGroup) Valid() bool {
	switch g {
	case GroupProject, GroupUser, GroupMilestone, GroupTask, GroupDay:
		return true
	}
	return false
}

// TimeReportFilter selects time logs for the time report.
type TimeReportFilter struct {
	From, To    Date
	ProjectIDs  []int64
	UserIDs     []int64
	MilestoneID int64
	Billable    *bool

	// Set by the use case from the viewer.
	OrgID     int64 // the organization whose projects count
	VisibleTo int64 // 0: every project; otherwise only projects this user is a member of
	OnlyUser  int64 // 0: everyone's logs; otherwise only this user's
}

// ValidPeriod checks a report period: from ≤ to, at most MaxReportDays long.
func ValidPeriod(from, to Date) error {
	if to.Before(from.Time) || to.Sub(from.Time) > MaxReportDays*24*time.Hour {
		return ErrReportPeriod
	}
	return nil
}

// TimeSum is one row of the time report.
type TimeSum struct {
	ID              int64 // project, user, milestone or card id; 0 for "no milestone" and for days
	Name            string
	Day             *Date // GroupDay only
	Minutes         int
	BillableMinutes int
	Count           int
}

// PersonProjectTime is someone's time on one project within a period.
type PersonProjectTime struct {
	UserID          int64
	ProjectID       int64
	ProjectName     string
	ProjectBillable bool
	Minutes         int
	BillableMinutes int
	BenchMinutes    int // logged on tasks of the special type "bench"
}

// PersonReport is one row of the billability and bench reports.
type PersonReport struct {
	User            User
	NormMinutes     int // working days in the period × working hours
	PastNormMinutes int // the same for days up to today, without absences
	Absence         map[VacationType]int
	Minutes         int
	BillableMinutes int
	BenchMinutes    int
	Projects        []PersonProjectTime
}

// AbsenceMinutes sums vacations, sick days and days off.
func (p PersonReport) AbsenceMinutes() int {
	n := 0
	for _, m := range p.Absence {
		n += m
	}
	return n
}

// BenchMinutesByNorm is the norm that is neither billable nor an absence (OmiTeam's "time without
// billable"); it can be negative when billable time and absences exceed the norm.
func (p PersonReport) BenchMinutesByNorm() int {
	return p.NormMinutes - p.BillableMinutes - p.AbsenceMinutes()
}

// NotTrackedMinutes: the norm of days that have already come, minus everything logged.
func (p PersonReport) NotTrackedMinutes() int { return max(0, p.PastNormMinutes-p.Minutes) }

// OverNormMinutes: logged time beyond the norm.
func (p PersonReport) OverNormMinutes() int { return max(0, p.Minutes-p.NormMinutes) }

// Task statistics of a project.

type StatsPeriod string

const (
	StatsWeek  StatsPeriod = "week"
	StatsMonth StatsPeriod = "month"
	StatsYear  StatsPeriod = "year"
)

// ColumnMove is a task moving between columns (from the task history).
type ColumnMove struct {
	CardID   int64
	From, To int64
	At       time.Time
}

// CardStart is when a task was created and where it is now.
type CardStart struct {
	CardID    int64
	ColumnID  int64
	CreatedAt time.Time
}

// StatsBucket is one period of the task statistics.
type StatsBucket struct {
	Start      Date
	Created    int // tasks created
	Done       int // tasks moved into a "done" column
	Stays      int // tasks that left the chosen columns
	AvgMinutes int // their average time in the chosen columns
}

type TaskStats struct {
	Buckets    []StatsBucket
	Stays      int
	AvgMinutes int // over all stays, not an average of averages
}
