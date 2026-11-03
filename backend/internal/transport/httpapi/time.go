package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/absence"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/timetracking"
)

func init() { registerRoutes(NewTimeHandler) }

// TimeHandler serves time tracking, absence requests and public holidays.
type TimeHandler struct {
	time    *timetracking.Service
	absence *absence.Service
	present *Presenter
}

func NewTimeHandler(t *timetracking.Service, a *absence.Service, present *Presenter) *TimeHandler {
	return &TimeHandler{time: t, absence: a, present: present}
}

func (h *TimeHandler) Mount(_, private *echo.Group) {
	private.GET("/timelogs/week", h.week)
	private.GET("/timelogs/missing", h.missing)
	private.POST("/timelogs", h.createLog)
	private.PUT("/timelogs/:id", h.updateLog)
	private.DELETE("/timelogs/:id", h.deleteLog)
	private.GET("/cards/:id/timelogs", h.cardLogs)

	private.GET("/holidays", h.holidays)
	private.POST("/holidays", h.saveHoliday)
	private.PUT("/holidays/:id", h.saveHoliday)
	private.DELETE("/holidays/:id", h.deleteHoliday)

	private.GET("/vacations/reviewers", h.reviewers)
	private.GET("/vacations/mine", h.myVacations)
	private.GET("/vacations/review", h.toReview)
	private.GET("/vacations/week", h.vacationsThisWeek)
	private.GET("/vacations/stats", h.vacationStats)
	private.GET("/vacations", h.allVacations)
	private.POST("/vacations", h.createVacation)
	private.PUT("/vacations/:id", h.updateVacation)
	private.PUT("/vacations/:id/reviewers", h.setReviewers)
	private.PUT("/vacations/:id/status", h.decide)
	private.DELETE("/vacations/:id", h.deleteVacation)
}

func dateParam(c *echo.Context, name string) (domain.Date, error) {
	v := c.QueryParam(name)
	if v == "" {
		return domain.Today(), nil
	}
	return domain.ParseDate(v)
}

func intParam(c *echo.Context, name string, fallback int) int {
	if n, err := strconv.Atoi(c.QueryParam(name)); err == nil {
		return n
	}
	return fallback
}

// --- time logs ---

func (h *TimeHandler) week(c *echo.Context) error {
	day, err := dateParam(c, "date")
	if err != nil {
		return err
	}
	userID, _ := strconv.ParseInt(c.QueryParam("user"), 10, 64)
	logs, stats, err := h.time.Week(c.Request().Context(), currentUser(c), userID, day)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, weekDTO{Logs: mapSlice(logs, toTimelog), Stats: toWeekStats(stats)})
}

func (h *TimeHandler) missing(c *echo.Context) error {
	days, err := h.time.MissingDays(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"days": days})
}

type timelogInput struct {
	UserID  int64       `json:"userId"`
	CardID  int64       `json:"cardId"`
	Day     domain.Date `json:"day"`
	Minutes int         `json:"minutes"`
	Comment string      `json:"comment"`
}

func (in timelogInput) domain() domain.TimelogInput {
	return domain.TimelogInput{UserID: in.UserID, CardID: in.CardID, Day: in.Day, Minutes: in.Minutes, Comment: in.Comment}
}

func (h *TimeHandler) createLog(c *echo.Context) error {
	var in timelogInput
	if err := bind(c, &in); err != nil {
		return err
	}
	t, err := h.time.Create(c.Request().Context(), currentUser(c), in.domain())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toTimelog(t))
}

func (h *TimeHandler) updateLog(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in timelogInput
		if err := bind(c, &in); err != nil {
			return err
		}
		t, err := h.time.Update(c.Request().Context(), currentUser(c), id, in.domain())
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toTimelog(t))
	})
}

func (h *TimeHandler) deleteLog(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.time.Delete(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

func (h *TimeHandler) cardLogs(c *echo.Context) error {
	return withID(c, func(id int64) error {
		logs, err := h.time.CardLogs(c.Request().Context(), currentUser(c), id, intParam(c, "offset", 0))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(logs, toTimelog))
	})
}

// --- holidays ---

func (h *TimeHandler) holidays(c *echo.Context) error {
	list, err := h.absence.Holidays(c.Request().Context(), currentUser(c), intParam(c, "year", time.Now().Year()))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, toHoliday))
}

func (h *TimeHandler) saveHoliday(c *echo.Context) error {
	var in holidayDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	if c.Param("id") != "" {
		id, err := pathID(c, "id")
		if err != nil {
			return err
		}
		in.ID = id
	} else {
		in.ID = 0
	}
	hol, err := h.absence.SaveHoliday(c.Request().Context(), currentUser(c), domain.Holiday{ID: in.ID, Day: in.Day, NameLatin: in.NameLatin, NameCyrillic: in.NameCyrillic})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toHoliday(hol))
}

func (h *TimeHandler) deleteHoliday(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.absence.DeleteHoliday(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

// --- vacations ---

func (h *TimeHandler) reviewers(c *echo.Context) error {
	users, err := h.absence.Reviewers(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(users, h.present.User))
}

func (h *TimeHandler) myVacations(c *echo.Context) error {
	list, err := h.absence.Mine(c.Request().Context(), currentUser(c), intParam(c, "year", time.Now().Year()), domain.VacationType(c.QueryParam("type")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, toVacation))
}

func (h *TimeHandler) toReview(c *echo.Context) error {
	list, err := h.absence.ToReview(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, toVacation))
}

func (h *TimeHandler) vacationsThisWeek(c *echo.Context) error {
	list, err := h.absence.ThisWeek(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, toVacation))
}

func (h *TimeHandler) vacationStats(c *echo.Context) error {
	userID, _ := strconv.ParseInt(c.QueryParam("user"), 10, 64)
	st, err := h.absence.Stats(c.Request().Context(), currentUser(c), userID, intParam(c, "year", time.Now().Year()))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"year": st.Year, "taken": st.Taken, "left": st.Left, "allowance": domain.VacationAllowance})
}

func (h *TimeHandler) allVacations(c *echo.Context) error {
	list, err := h.absence.All(c.Request().Context(), currentUser(c), domain.VacationFilter{
		Year: intParam(c, "year", time.Now().Year()), Month: intParam(c, "month", 0),
		Status: domain.VacationStatus(c.QueryParam("status")), Type: domain.VacationType(c.QueryParam("type")),
		RequesterName: c.QueryParam("name"),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, toVacation))
}

type vacationInput struct {
	Type        domain.VacationType `json:"type"`
	Start       domain.Date         `json:"start"`
	End         domain.Date         `json:"end"`
	Comment     string              `json:"comment"`
	ReviewerIDs []int64             `json:"reviewerIds"`
}

func (in vacationInput) domain() absence.RequestInput {
	return absence.RequestInput{Type: in.Type, Start: in.Start, End: in.End, Comment: in.Comment, ReviewerIDs: in.ReviewerIDs}
}

func (h *TimeHandler) createVacation(c *echo.Context) error {
	var in vacationInput
	if err := bind(c, &in); err != nil {
		return err
	}
	v, err := h.absence.Create(c.Request().Context(), currentUser(c), in.domain())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toVacation(v))
}

func (h *TimeHandler) updateVacation(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in vacationInput
		if err := bind(c, &in); err != nil {
			return err
		}
		v, err := h.absence.Update(c.Request().Context(), currentUser(c), id, in.domain())
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toVacation(v))
	})
}

func (h *TimeHandler) setReviewers(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			ReviewerIDs []int64 `json:"reviewerIds"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		v, err := h.absence.SetReviewers(c.Request().Context(), currentUser(c), id, in.ReviewerIDs)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toVacation(v))
	})
}

func (h *TimeHandler) decide(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Status domain.VacationStatus `json:"status"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		v, err := h.absence.Decide(c.Request().Context(), currentUser(c), id, in.Status)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toVacation(v))
	})
}

func (h *TimeHandler) deleteVacation(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.absence.Delete(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

// --- shapes ---

type timelogDTO struct {
	ID          int64       `json:"id"`
	UserID      int64       `json:"userId"`
	UserName    string      `json:"userName"`
	CardID      int64       `json:"cardId"`
	CardKey     string      `json:"cardKey"`
	CardTitle   string      `json:"cardTitle"`
	ProjectID   int64       `json:"projectId"`
	ProjectName string      `json:"projectName"`
	Day         domain.Date `json:"day"`
	Minutes     int         `json:"minutes"`
	Comment     string      `json:"comment"`
	Billable    bool        `json:"billable"`
	CreatedBy   *int64      `json:"createdBy"`
}

func toTimelog(t domain.Timelog) timelogDTO {
	return timelogDTO{t.ID, t.UserID, t.UserName, t.CardID, t.CardKey, t.CardTitle, t.ProjectID, t.ProjectName, t.Day, t.Minutes, t.Comment, t.Billable, t.CreatedBy}
}

type dayStatsDTO struct {
	Day             domain.Date `json:"day"`
	Minutes         int         `json:"minutes"`
	BillableMinutes int         `json:"billableMinutes"`
	NormMinutes     int         `json:"normMinutes"`
	Holiday         string      `json:"holiday"`
	Absence         string      `json:"absence"`
}

type weekStatsDTO struct {
	Days            []dayStatsDTO `json:"days"`
	Minutes         int           `json:"minutes"`
	BillableMinutes int           `json:"billableMinutes"`
	NormMinutes     int           `json:"normMinutes"`
	OvertimeMinutes int           `json:"overtimeMinutes"`
}

type weekDTO struct {
	Logs  []timelogDTO `json:"logs"`
	Stats weekStatsDTO `json:"stats"`
}

func toWeekStats(w domain.WeekStats) weekStatsDTO {
	return weekStatsDTO{
		Days: mapSlice(w.Days, func(d domain.DayStats) dayStatsDTO {
			return dayStatsDTO{d.Day, d.Minutes, d.BillableMinutes, d.NormMinutes, d.Holiday, string(d.Absence)}
		}),
		Minutes: w.Minutes, BillableMinutes: w.BillableMinutes, NormMinutes: w.NormMinutes, OvertimeMinutes: w.OvertimeMinutes,
	}
}

type holidayDTO struct {
	ID           int64       `json:"id"`
	Day          domain.Date `json:"day"`
	NameLatin    string      `json:"nameLatin"`
	NameCyrillic string      `json:"nameCyrillic"`
}

func toHoliday(h domain.Holiday) holidayDTO {
	return holidayDTO{h.ID, h.Day, h.NameLatin, h.NameCyrillic}
}

type vacationDTO struct {
	ID          int64                 `json:"id"`
	UserID      int64                 `json:"userId"`
	UserName    string                `json:"userName"`
	Type        domain.VacationType   `json:"type"`
	Start       domain.Date           `json:"start"`
	End         domain.Date           `json:"end"`
	Days        int                   `json:"days"`
	Comment     string                `json:"comment"`
	Status      domain.VacationStatus `json:"status"`
	DecidedBy   *int64                `json:"decidedBy"`
	DecidedAt   *time.Time            `json:"decidedAt"`
	CreatedAt   time.Time             `json:"createdAt"`
	ReviewerIDs []int64               `json:"reviewerIds"`
}

func toVacation(v domain.Vacation) vacationDTO {
	return vacationDTO{v.ID, v.UserID, v.UserName, v.Type, v.Start, v.End, v.Days, v.Comment, v.Status, v.DecidedBy, v.DecidedAt,
		v.CreatedAt, nonNil(v.ReviewerIDs)}
}
