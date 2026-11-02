package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
)

// Delivery of what arrives from Redis (other instances and use cases) to this instance's connections,
// and the periodic presence / voice heartbeats.

// dispatch fans messages from Redis out to this instance's connections.
func (h *Hub) dispatch(msgs <-chan collab.Message) {
	for m := range msgs {
		if m.SignalTo != "" {
			if c := h.conn(m.SignalTo); c != nil {
				h.deliverSignal(c, m.Payload)
			}
			continue
		}
		if e := m.UserEvent; e != nil {
			ev := map[string]any{"type": e.Type, "reason": e.Reason, "orgId": e.OrgID, "projectId": e.ProjectID, "name": e.Name, "actorId": e.ActorID}
			if e.Type == domain.UserVoiceChanged {
				ev["participants"] = nonNil(e.Voice)
			}
			payload, _ := json.Marshal(map[string]any{"type": "user", "event": ev})
			for _, c := range h.userClients(e.UserID) {
				c.enqueue(payload)
			}
			continue
		}
		var payload []byte
		if m.Event != nil {
			payload, _ = json.Marshal(map[string]any{"type": "event", "event": h.eventDTO(*m.Event)})
		} else {
			payload = m.Payload
		}
		for _, c := range h.clients(m.ProjectID) {
			if m.Event == nil && c.sess.ConnID == m.From {
				continue
			}
			c.enqueue(payload)
			// Nobody keeps listening to a board they can no longer open.
			if e := m.Event; e != nil && (e.Type == domain.EventProjectDeleted || (e.RemovedUserID != 0 && e.RemovedUserID == c.sess.User.ID)) {
				c.revoke()
			}
		}
		if e := m.Event; e != nil && (e.Type == domain.EventProjectDeleted || e.RemovedUserID != 0) {
			h.revokeVoice(*e)
		}
	}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (h *Hub) eventDTO(e domain.BoardEvent) map[string]any {
	ev := map[string]any{"type": e.Type, "actorId": e.ActorID, "origin": e.Origin, "removedUserId": e.RemovedUserID}
	if e.Card != nil {
		ev["card"] = h.present.Card(*e.Card)
	}
	switch e.Type {
	case domain.EventCardDeleted:
		ev["cardId"], ev["columnId"] = e.CardID, e.ColumnID
	case domain.EventCardMoved:
		ev["fromColumnId"], ev["afterId"], ev["beforeId"] = e.FromColumnID, e.AfterID, e.BeforeID
	}
	return ev
}

func (h *Hub) heartbeats(ctx context.Context) {
	t := time.NewTicker(heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.mu.RLock()
			var all []*wsClient
			for _, room := range h.rooms {
				for c := range room {
					all = append(all, c)
				}
			}
			h.mu.RUnlock()
			for _, c := range all {
				c.mu.Lock()
				focus := c.focus
				c.mu.Unlock()
				if err := h.collab.Heartbeat(ctx, c.sess, focus); err != nil {
					h.log.Warn("presence heartbeat", "err", err)
				}
			}
			for _, c := range h.voiceClients() {
				room, muted := c.voiceState()
				if err := h.voice.Heartbeat(ctx, c.sess.User, c.sess.ConnID, room, muted); err != nil {
					h.log.Warn("voice heartbeat", "err", err)
				}
			}
		}
	}
}
