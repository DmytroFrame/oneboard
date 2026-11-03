package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/reports"
)

func init() { registerRoutes(NewReportHandler) }

// ReportHandler serves the time, billability and bench reports and a project's task statistics.
type ReportHandler struct {
	reports *reports.Service
	present *Presenter
}

func NewReportHandler(r *reports.Service, present *Presenter) *ReportHandler {
	return &ReportHandler{reports: r, present: present}
}

func (h *ReportHandler) Mount(_, private *echo.Group) {
	private.GET("/reports/time", h.time)
	private.GET("/reports/time/export", h.export)
	private.GET("/reports/people", h.people)
	private.GET("/projects/:id/statistics", h.taskStats)
}

// period reads from/to (YYYY-MM-DD); both are required.
func period(c *echo.Context) (domain.Date, domain.Date, error) {
	from, err := domain.ParseDate(c.QueryParam("from"))
	if err != nil {
		return from, from, domain.ErrReportPeriod
	}
	to, err := domain.ParseDate(c.QueryParam("to"))
	if err != nil {
		return from, to, domain.ErrReportPeriod
	}
	return from, to, nil
}

func timeFilter(c *echo.Context) (domain.TimeReportFilter, error) {
	from, to, err := period(c)
	f := domain.TimeReportFilter{From: from, To: to, ProjectIDs: idList(c.QueryParam("projects")), UserIDs: idList(c.QueryParam("users"))}
	if m := optID(c.QueryParam("milestone")); m != nil {
		f.MilestoneID = *m
	}
	if b, perr := strconv.ParseBool(c.QueryParam("billable")); perr == nil {
		f.Billable = &b
	}
	return f, err
}

func (h *ReportHandler) time(c *echo.Context) error {
	f, err := timeFilter(c)
	if err != nil {
		return err
	}
	rows, err := h.reports.Time(c.Request().Context(), currentUser(c), f, domain.ReportGroup(c.QueryParam("group")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(rows, timeSumDTOOf))
}

// export downloads the logs behind the time report as CSV (opens in Excel thanks to the BOM).
func (h *ReportHandler) export(c *echo.Context) error {
	f, err := timeFilter(c)
	if err != nil {
		return err
	}
	logs, err := h.reports.TimeLogs(c.Request().Context(), currentUser(c), f)
	if err != nil {
		return err
	}
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="time-report_%s_%s.csv"`, f.From, f.To))
	res.WriteHeader(http.StatusOK)
	_, _ = res.Write([]byte("\xef\xbb\xbf"))
	w := csv.NewWriter(res)
	_ = w.Write([]string{"Date", "Person", "Project", "Task", "Title", "Hours", "Billable", "Comment"})
	for _, l := range logs {
		_ = w.Write([]string{l.Day.String(), csvText(l.UserName), csvText(l.ProjectName), l.CardKey, csvText(l.CardTitle),
			strconv.FormatFloat(float64(l.Minutes)/60, 'f', 2, 64), strconv.FormatBool(l.Billable), csvText(l.Comment)})
	}
	w.Flush()
	return w.Error()
}

// csvText keeps spreadsheets from running user text as a formula (CSV injection).
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (h *ReportHandler) people(c *echo.Context) error {
	from, to, err := period(c)
	if err != nil {
		return err
	}
	var roles []domain.CompanyRole
	for _, r := range strings.Split(c.QueryParam("roles"), ",") {
		if r = strings.TrimSpace(r); r != "" {
			roles = append(roles, domain.CompanyRole(r))
		}
	}
	rows, err := h.reports.People(c.Request().Context(), currentUser(c), from, to, roles)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(rows, h.present.PersonReport))
}

func (h *ReportHandler) taskStats(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	stats, err := h.reports.TaskStats(c.Request().Context(), currentUser(c), id,
		domain.StatsPeriod(c.QueryParam("period")), idList(c.QueryParam("columns")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, taskStatsDTOOf(stats))
}
