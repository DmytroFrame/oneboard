package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// Voice rooms over the user channel: join/leave/mute/signal from a tab, signals addressed to one
// connection, and taking people out of a room when they lose access.

// handleVoice handles a user-channel message: everything a tab says there is about voice rooms.
func (h *Hub) handleVoice(ctx context.Context, c *wsClient, m wsMessage) error {
	uid, conn := c.sess.User.ID, c.sess.ConnID
	room, muted := c.voiceState()
	switch m.Type {
	case "voice.join":
		if room != 0 && room != m.ProjectID {
			if err := h.voice.Leave(ctx, uid, conn, room); err != nil {
				return err
			}
			c.setVoice(0, false)
		}
		participants, err := h.voice.Join(ctx, c.sess.User, conn, m.ProjectID, m.Muted)
		if err != nil {
			code := "voice_failed"
			if e, ok := domain.AsError(err); ok {
				code = e.Code
			}
			c.sendJSON(map[string]any{"type": "voice.error", "code": code, "projectId": m.ProjectID})
			return nil
		}
		c.setVoice(m.ProjectID, m.Muted)
		c.sendJSON(map[string]any{"type": "voice.joined", "projectId": m.ProjectID, "participants": nonNil(participants)})
		return nil

	case "voice.leave":
		if room == 0 {
			return nil
		}
		c.setVoice(0, false)
		return h.voice.Leave(ctx, uid, conn, room)

	case "voice.mute":
		if room == 0 || muted == m.Muted {
			return nil
		}
		c.setVoice(room, m.Muted)
		return h.voice.SetMuted(ctx, c.sess.User, conn, room, m.Muted)

	case "voice.signal":
		if room == 0 || m.To == "" || len(m.Signal) == 0 || len(m.Signal) > 64<<10 {
			return nil
		}
		return h.voice.Signal(ctx, uid, conn, room, m.To, m.Signal)
	}
	return nil
}

// deliverSignal passes a message addressed to one connection; "voice.left" (the user joined from
// another tab or board) also means this tab is no longer in its room.
func (h *Hub) deliverSignal(c *wsClient, payload []byte) {
	var head struct {
		Type      string `json:"type"`
		ProjectID int64  `json:"projectId"`
	}
	if json.Unmarshal(payload, &head) == nil && head.Type == "voice.left" {
		c.mu.Lock()
		if c.voiceRoom == head.ProjectID {
			c.voiceRoom, c.voiceMuted = 0, false
		}
		c.mu.Unlock()
	}
	c.enqueue(payload)
}

// revokeVoice takes people who lost access to a project out of its voice room.
func (h *Hub) revokeVoice(e domain.BoardEvent) {
	for _, c := range h.voiceClients() {
		room, _ := c.voiceState()
		if room != e.ProjectID || (e.Type != domain.EventProjectDeleted && c.sess.User.ID != e.RemovedUserID) {
			continue
		}
		c.setVoice(0, false)
		c.sendJSON(map[string]any{"type": "voice.left", "reason": "revoked", "projectId": room})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.voice.Leave(ctx, c.sess.User.ID, c.sess.ConnID, room); err != nil {
				h.log.Warn("voice revoke", "err", err)
			}
		}()
	}
}
