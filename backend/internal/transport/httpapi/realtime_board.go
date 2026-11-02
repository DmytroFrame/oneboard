package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// Messages a client sends on a board connection: cursors, drags, focus and the card documents.

// checkCard verifies (once per connection) that the card is on this board.
func (h *Hub) checkCard(ctx context.Context, c *wsClient, cardID *int64) error {
	if cardID == nil {
		return errors.New("cardId required")
	}
	c.mu.Lock()
	ok := c.cards[*cardID]
	c.mu.Unlock()
	if ok {
		return nil
	}
	if err := h.collab.CheckCard(ctx, c.sess, *cardID); err != nil {
		return err
	}
	c.mu.Lock()
	c.cards[*cardID] = true
	c.mu.Unlock()
	return nil
}

func (h *Hub) relay(ctx context.Context, c *wsClient, msg map[string]any) error {
	msg["connId"] = c.sess.ConnID
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return h.collab.RelayRaw(ctx, c.sess, b)
}

func (h *Hub) handle(ctx context.Context, c *wsClient, m wsMessage) error {
	if c.sess.UserOnly() {
		return h.handleVoice(ctx, c, m)
	}
	switch m.Type {
	case "cursor":
		c.mu.Lock()
		tooSoon := time.Since(c.lastCursor) < cursorInterval
		if !tooSoon {
			c.lastCursor = time.Now()
		}
		c.mu.Unlock()
		if tooSoon {
			return nil
		}
		return h.relay(ctx, c, map[string]any{"type": "cursor", "anchor": m.Anchor})

	case "drag":
		if m.CardID != nil {
			if err := h.checkCard(ctx, c, m.CardID); err != nil {
				return err
			}
		}
		return h.relay(ctx, c, map[string]any{"type": "drag", "cardId": m.CardID, "anchor": m.Anchor})

	case "focus":
		if m.CardID != nil {
			if err := h.checkCard(ctx, c, m.CardID); err != nil {
				return err
			}
		}
		c.mu.Lock()
		c.focus = m.CardID
		c.mu.Unlock()
		return h.collab.Focus(ctx, c.sess, m.CardID)

	case "doc.open":
		if err := h.checkCard(ctx, c, m.CardID); err != nil {
			return err
		}
		init, err := h.collab.OpenDoc(ctx, c.sess, *m.CardID)
		if err != nil {
			return err
		}
		updates := make([]string, len(init.Updates))
		for i, u := range init.Updates {
			updates[i] = base64.StdEncoding.EncodeToString(u)
		}
		c.sendJSON(map[string]any{"type": "doc.init", "cardId": *m.CardID, "base": init.Base, "updates": updates, "seed": init.Seed})
		return nil

	case "doc.update":
		if err := h.checkCard(ctx, c, m.CardID); err != nil {
			return err
		}
		update, err := base64.StdEncoding.DecodeString(m.Update)
		if err != nil || len(update) == 0 {
			return errors.New("bad update")
		}
		seq, err := h.collab.AppendDoc(ctx, *m.CardID, update)
		if err != nil {
			return err
		}
		c.sendJSON(map[string]any{"type": "doc.ack", "cardId": *m.CardID, "seq": seq})
		return h.relay(ctx, c, map[string]any{"type": "doc.update", "cardId": *m.CardID, "update": m.Update, "seq": seq})

	case "doc.compact":
		if err := h.checkCard(ctx, c, m.CardID); err != nil {
			return err
		}
		state, err := base64.StdEncoding.DecodeString(m.State)
		if err != nil || len(state) == 0 || m.Upto <= 0 {
			return errors.New("bad snapshot")
		}
		return h.collab.CompactDoc(ctx, *m.CardID, m.Upto, state)

	case "awareness":
		if err := h.checkCard(ctx, c, m.CardID); err != nil {
			return err
		}
		return h.relay(ctx, c, map[string]any{"type": "awareness", "cardId": *m.CardID, "data": m.Data})
	}
	return nil
}
