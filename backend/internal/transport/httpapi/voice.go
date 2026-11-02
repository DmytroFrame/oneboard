package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
)

func init() { registerRoutes(NewVoiceHandler) }

// VoiceHandler serves what a tab needs before joining a voice room over the WebSocket.
type VoiceHandler struct{ voice *voice.Service }

func NewVoiceHandler(v *voice.Service) *VoiceHandler { return &VoiceHandler{voice: v} }

func (h *VoiceHandler) Mount(_, private *echo.Group) {
	private.GET("/voice/rooms", h.rooms)
	private.GET("/voice/ice", h.ice)
}

// rooms returns who is in the voice rooms of the user's projects (only non-empty rooms).
func (h *VoiceHandler) rooms(c *echo.Context) error {
	rooms, err := h.voice.Rooms(c.Request().Context(), currentUser(c).ID)
	if err != nil {
		return err
	}
	out := map[string]any{}
	for id, ps := range rooms {
		out[strconv.FormatInt(id, 10)] = ps
	}
	return c.JSON(http.StatusOK, map[string]any{"rooms": out})
}

// ice returns the STUN/TURN servers for RTCPeerConnection, with short-lived TURN credentials.
func (h *VoiceHandler) ice(c *echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]any{"iceServers": h.voice.ICEServers(currentUser(c).ID, time.Now())})
}
