package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
)

// VoiceHandler serves what a tab needs before joining a voice room over the WebSocket.
type VoiceHandler struct{ voice *voice.Service }

func NewVoiceHandler(v *voice.Service) *VoiceHandler { return &VoiceHandler{voice: v} }

func (h *VoiceHandler) Mount(_, private chi.Router) {
	private.Get("/voice/rooms", h.rooms)
	private.Get("/voice/ice", h.ice)
}

// rooms returns who is in the voice rooms of the user's projects (only non-empty rooms).
func (h *VoiceHandler) rooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.voice.Rooms(r.Context(), currentUser(r).ID)
	if err != nil {
		fail(w, r, err)
		return
	}
	out := map[string]any{}
	for id, ps := range rooms {
		out[strconv.FormatInt(id, 10)] = ps
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": out})
}

// ice returns the STUN/TURN servers for RTCPeerConnection, with short-lived TURN credentials.
func (h *VoiceHandler) ice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"iceServers": h.voice.ICEServers(currentUser(r).ID, time.Now())})
}
