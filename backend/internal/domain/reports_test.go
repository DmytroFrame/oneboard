package domain

import "testing"

func TestPersonReportFormulas(t *testing.T) {
	r := PersonReport{
		NormMinutes: 10 * 480, PastNormMinutes: 6 * 480, Minutes: 5 * 480, BillableMinutes: 3 * 480,
		Absence: map[VacationType]int{VacationRegular: 2 * 480, VacationSick: 480},
	}
	if got := r.BenchMinutesByNorm(); got != 4*480 {
		t.Errorf("bench by norm = %d, want %d", got, 4*480)
	}
	if got := r.NotTrackedMinutes(); got != 480 {
		t.Errorf("not tracked = %d, want 480", got)
	}
	if got := r.OverNormMinutes(); got != 0 {
		t.Errorf("over norm = %d, want 0", got)
	}
	r.BillableMinutes = 9 * 480 // billable + absences above the norm: negative bench
	if got := r.BenchMinutesByNorm(); got != -2*480 {
		t.Errorf("bench by norm = %d, want %d", got, -2*480)
	}
}

func TestValidPeriod(t *testing.T) {
	from := NewDate(2026, 1, 1)
	if ValidPeriod(from, from.AddDays(MaxReportDays)) != nil {
		t.Error("a period of MaxReportDays must be valid")
	}
	if ValidPeriod(from, from.AddDays(MaxReportDays+1)) == nil || ValidPeriod(from, from.AddDays(-1)) == nil {
		t.Error("too long or reversed periods must be rejected")
	}
}
