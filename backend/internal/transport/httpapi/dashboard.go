package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

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
}

func (h *DashboardHandler) get(c *echo.Context) error {
	d, err := h.dashboard.Get(c.Request().Context(), currentUser(c).ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Dashboard(d))
}
