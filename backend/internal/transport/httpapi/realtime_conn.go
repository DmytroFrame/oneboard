package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
)

// One WebSocket connection: authentication, the read loop and the write loop.

type wsClient struct {
	sess collab.Session
	conn *websocket.Conn
	send chan []byte

	mu         sync.Mutex
	focus      *int64
	cards      map[int64]bool // cards already checked to belong to the board
	lastCursor time.Time
	voiceRoom  int64 // user channel: the project whose voice room this tab is in (0 = none)
	voiceMuted bool
}

func (c *wsClient) voiceState() (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.voiceRoom, c.voiceMuted
}

func (c *wsClient) setVoice(projectID int64, muted bool) {
	c.mu.Lock()
	c.voiceRoom, c.voiceMuted = projectID, muted
	c.mu.Unlock()
}

// revoke closes a connection that lost access, shortly after the event explaining why was queued.
func (c *wsClient) revoke() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = c.conn.Close(websocket.StatusPolicyViolation, "access revoked")
	}()
}

// enqueue never blocks: a client that can't keep up is disconnected (it will reconnect and resync).
func (c *wsClient) enqueue(msg []byte) {
	select {
	case c.send <- msg:
	default:
		go c.conn.Close(websocket.StatusPolicyViolation, "too slow")
	}
}

func (c *wsClient) sendJSON(v any) {
	if b, err := json.Marshal(v); err == nil {
		c.enqueue(b)
	}
}

type wsMessage struct {
	Type      string          `json:"type"`
	Token     string          `json:"token,omitempty"`
	ProjectID int64           `json:"projectId,omitempty"`
	CardID    *int64          `json:"cardId,omitempty"`
	Anchor    json.RawMessage `json:"anchor,omitempty"`
	Update    string          `json:"update,omitempty"` // base64 Yjs update
	Upto      int64           `json:"upto,omitempty"`
	State     string          `json:"state,omitempty"` // base64 Yjs snapshot
	Data      string          `json:"data,omitempty"`  // base64 awareness update
	Muted     bool            `json:"muted,omitempty"`
	To        string          `json:"to,omitempty"`     // voice.signal: the participant's connection id
	Signal    json.RawMessage `json:"signal,omitempty"` // voice.signal: SDP description or ICE candidate, relayed as is
}

func newConnID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Hub) serve(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin handshakes by default (Origin must match Host).
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(2 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sess, peers, err := h.authenticate(ctx, conn)
	if err != nil {
		code := "not_authenticated"
		if e, ok := domain.AsError(err); ok {
			code = e.Code
		}
		b, _ := json.Marshal(map[string]string{"type": "error", "code": code})
		_ = conn.Write(ctx, websocket.MessageText, b)
		_ = conn.Close(websocket.StatusPolicyViolation, code)
		return
	}

	c := &wsClient{sess: sess, conn: conn, send: make(chan []byte, sendBuffer), cards: map[int64]bool{}}
	h.add(c)
	defer func() {
		h.remove(c)
		leaveCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := h.collab.Leave(leaveCtx, sess); err != nil {
			h.log.Warn("presence leave", "err", err)
		}
		if room, _ := c.voiceState(); room != 0 {
			if err := h.voice.Leave(leaveCtx, sess.User.ID, sess.ConnID, room); err != nil {
				h.log.Warn("voice leave", "err", err)
			}
		}
	}()
	if peers == nil {
		peers = []domain.Peer{}
	}
	c.sendJSON(map[string]any{"type": "welcome", "connId": sess.ConnID, "peers": peers})

	go h.writeLoop(ctx, cancel, c)
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var m wsMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if err := h.handle(ctx, c, m); err != nil {
			h.log.Warn("realtime message", "type", m.Type, "err", err)
		}
	}
}

func (h *Hub) authenticate(ctx context.Context, conn *websocket.Conn) (collab.Session, []domain.Peer, error) {
	authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, data, err := conn.Read(authCtx)
	if err != nil {
		return collab.Session{}, nil, err
	}
	var m wsMessage
	if json.Unmarshal(data, &m) != nil || m.Type != "auth" {
		return collab.Session{}, nil, domain.ErrNotAuthenticated
	}
	return h.collab.Join(ctx, m.Token, m.ProjectID, newConnID())
}

func (h *Hub) writeLoop(ctx context.Context, cancel context.CancelFunc, c *wsClient) {
	defer cancel()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.send:
			wctx, done := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			done()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, done := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pctx)
			done()
			if err != nil {
				return
			}
		}
	}
}
