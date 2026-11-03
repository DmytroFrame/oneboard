package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/dashboard"
)

func init() { registerRoutes(NewDashboardHandler) }

type DashboardHandler struct {
	dashboard *dashboard.Service
	present   *Presenter
}

func NewDashboardHandler(d *dashboard.Service, present *Presenter) *DashboardHandler {
	return &DashboardHandler{dashboard: d, present: present}
}

func (h *DashboardHandler) Mount(_, private *echo.Group) {
	private.GET("/dashboard", h.get)
	private.GET("/dashboard/company", h.company)
	private.GET("/dashboard/worktime", h.workTime)
}

func (h *DashboardHandler) get(c *echo.Context) error {
	d, err := h.dashboard.Get(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Dashboard(d))
}

func (h *DashboardHandler) company(c *echo.Context) error {
	co, err := h.dashboard.CompanyFor(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Company(co))
}

func (h *DashboardHandler) workTime(c *echo.Context) error {
	day, err := dateParam(c, "date")
	if err != nil {
		return err
	}
	days, err := h.dashboard.WorkTime(c.Request().Context(), currentUser(c), c.QueryParam("team") == "1", day)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(days, func(d domain.DayWork) map[string]any {
		return map[string]any{"day": d.Day, "billableMinutes": d.BillableMinutes, "otherMinutes": d.OtherMinutes}
	}))
}
