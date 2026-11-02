package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/fx"

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

// The hub is split by concern: this file holds the registry of connections, realtime_conn.go one
// connection's lifecycle, realtime_fanout.go delivery of messages from Redis, realtime_board.go and
// realtime_voice.go the messages a client sends on a board and on its user channel.

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

func init() { registerRoutes(NewHub) }

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

func (h *Hub) Mount(public, _ *echo.Group) {
	public.GET("/ws", echo.WrapHandler(http.HandlerFunc(h.serve))) // authenticated by the first message
}

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
