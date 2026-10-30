package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
)

// The real-time protocol is JSON over a WebSocket at /api/ws, one connection per open board.
//
//	client → {type:"auth", token, projectId}            first message (browsers can't set headers on WebSockets);
//	                                                     projectId 0 opens the user channel instead of a board
//	server → {type:"welcome", connId, peers}             or {type:"error", code} and close
//	client → cursor{anchor} drag{cardId,anchor} focus{cardId}
//	         doc.open{cardId} doc.update{cardId,update} doc.compact{cardId,upto,state} awareness{cardId,data}
//	server → the same messages from other peers (with connId), join/leave/focus,
//	         doc.init{cardId,base,updates,seed} doc.ack{cardId,seq}, event{event}
//
// User channel (one per tab, lives across pages):
//
//	server → user{event}                                 access changes; event.type "voice" carries a room's participants
//	client → voice.join{projectId,muted} voice.leave voice.mute{muted} voice.signal{to,signal}
//	server → voice.joined{projectId,participants} voice.error{code} voice.left{reason,projectId}
//	         voice.signal{from,userId,signal}             WebRTC offer/answer/candidate from another participant
//
// Peer messages, committed board events and voice signals reach every server instance through Redis pub/sub.

const (
	sendBuffer     = 256
	writeTimeout   = 5 * time.Second
	pingInterval   = 30 * time.Second
	heartbeatEvery = 20 * time.Second
	cursorInterval = 30 * time.Millisecond // server-side cap per connection
)

type Hub struct {
	collab  *collab.Service
	voice   *voice.Service
	present *Presenter
	log     *slog.Logger

	mu    sync.RWMutex
	rooms map[int64]map[*wsClient]struct{} // board connections by project
	users map[int64]map[*wsClient]struct{} // user-channel connections by user
	conns map[string]*wsClient             // every connection by id (voice signals are addressed to one)
}

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

func NewHub(lc fx.Lifecycle, c *collab.Service, v *voice.Service, present *Presenter, log *slog.Logger) *Hub {
	h := &Hub{
		collab: c, voice: v, present: present, log: log,
		rooms: map[int64]map[*wsClient]struct{}{}, users: map[int64]map[*wsClient]struct{}{}, conns: map[string]*wsClient{},
	}
	var cancel context.CancelFunc
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var ctx context.Context
			ctx, cancel = context.WithCancel(context.Background())
			msgs, err := c.Subscribe(ctx)
			if err != nil {
				cancel()
				return err
			}
			go h.dispatch(msgs)
			go h.heartbeats(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
	return h
}

func (h *Hub) Mount(public, _ chi.Router) {
	public.Get("/ws", h.serve) // authenticated by the first message
}

// ---------- rooms ----------

// registry picks the map a connection lives in: its board, or its user's channel.
func (h *Hub) registry(c *wsClient) (map[int64]map[*wsClient]struct{}, int64) {
	if c.sess.UserOnly() {
		return h.users, c.sess.User.ID
	}
	return h.rooms, c.sess.ProjectID
}

func (h *Hub) add(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	reg, key := h.registry(c)
	if reg[key] == nil {
		reg[key] = map[*wsClient]struct{}{}
	}
	reg[key][c] = struct{}{}
	h.conns[c.sess.ConnID] = c
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c.sess.ConnID)
	reg, key := h.registry(c)
	if set := reg[key]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(reg, key)
		}
	}
}

func list(set map[*wsClient]struct{}) []*wsClient {
	out := make([]*wsClient, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func (h *Hub) clients(projectID int64) []*wsClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return list(h.rooms[projectID])
}

func (h *Hub) userClients(userID int64) []*wsClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return list(h.users[userID])
}

func (h *Hub) conn(connID string) *wsClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[connID]
}

// voiceClients returns this instance's tabs that are in a voice room.
func (h *Hub) voiceClients() []*wsClient {
	h.mu.RLock()
	var all []*wsClient
	for _, set := range h.users {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	return slices.DeleteFunc(all, func(c *wsClient) bool { room, _ := c.voiceState(); return room == 0 })
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

// ---------- connection ----------

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
