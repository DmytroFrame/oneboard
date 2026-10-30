// Package voice implements per-board voice rooms. Audio flows peer to peer over WebRTC; the server
// only keeps track of who is in which room, tells the project's members, and relays the WebRTC
// signaling (offers, answers, ICE candidates) between participants without looking inside it.
//
// A participant is a tab's user-channel connection, not a board connection, so someone stays in the
// room while browsing other pages — like a voice channel in Discord.
package voice

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = fx.Module("voice", fx.Provide(NewService))

// PresenceTTL is how long a participant stays listed without a heartbeat (e.g. after a server crash).
const PresenceTTL = 60 * time.Second

// Slot is where a user currently is in voice: one room, from one tab.
type Slot struct {
	ProjectID int64  `json:"projectId"`
	ConnID    string `json:"connId"`
}

// Rooms stores who is in each board's voice room.
type Rooms interface {
	Put(ctx context.Context, projectID int64, p domain.VoiceParticipant, ttl time.Duration) error
	Remove(ctx context.Context, projectID int64, connID string) error
	List(ctx context.Context, projectID int64) ([]domain.VoiceParticipant, error)
	// Claim records the user's slot and returns the one it replaces, if any (another tab or room).
	Claim(ctx context.Context, userID int64, slot Slot, ttl time.Duration) (*Slot, error)
	// Release forgets the user's slot if it still belongs to connID.
	Release(ctx context.Context, userID int64, connID string) error
}

// Signals delivers a message to one connection, on whichever server instance holds it.
type Signals interface {
	PublishSignal(ctx context.Context, toConnID string, payload []byte) error
}

type Members interface {
	List(ctx context.Context, projectID int64) ([]domain.Member, error)
}

type Projects interface {
	ListForUser(ctx context.Context, userID int64) ([]domain.Project, error)
}

type Service struct {
	rooms    Rooms
	signals  Signals
	events   usecase.UserEvents
	members  usecase.Memberships
	list     Members
	projects Projects
	ice      iceConfig
}

type Deps struct {
	fx.In
	Rooms       Rooms
	Signals     Signals
	Events      usecase.UserEvents
	Memberships usecase.Memberships
	Members     Members
	Projects    Projects
	Config      config.Config
}

func NewService(d Deps) *Service {
	return &Service{
		rooms: d.Rooms, signals: d.Signals, events: d.Events, members: d.Memberships, list: d.Members, projects: d.Projects,
		ice: iceConfig{stun: d.Config.STUNURLs, turn: d.Config.TURNURLs, secret: d.Config.TURNSecret, ttl: d.Config.TURNCredentialTTL},
	}
}

// Join puts the connection into the project's voice room and returns everyone there (including it).
// The user's previous slot — another room, or the same room from another tab — is vacated first, and
// that tab is told it was replaced.
func (s *Service) Join(ctx context.Context, u domain.User, connID string, projectID int64, muted bool) ([]domain.VoiceParticipant, error) {
	userID := u.ID
	if _, err := usecase.RequireMember(ctx, s.members, projectID, userID, domain.ErrProjectNotFound); err != nil {
		return nil, err
	}
	current, err := s.rooms.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	others := slices.DeleteFunc(slices.Clone(current), func(p domain.VoiceParticipant) bool {
		return p.ConnID == connID || p.UserID == userID
	})
	if len(others) >= domain.VoiceRoomLimit {
		return nil, domain.ErrVoiceRoomFull
	}

	prev, err := s.rooms.Claim(ctx, userID, Slot{ProjectID: projectID, ConnID: connID}, PresenceTTL)
	if err != nil {
		return nil, err
	}
	if prev != nil && *prev != (Slot{ProjectID: projectID, ConnID: connID}) {
		if err := s.rooms.Remove(ctx, prev.ProjectID, prev.ConnID); err != nil {
			return nil, err
		}
		if prev.ConnID != connID {
			_ = s.notify(ctx, prev.ConnID, map[string]any{"type": "voice.left", "reason": "elsewhere", "projectId": prev.ProjectID})
		}
		if prev.ProjectID != projectID {
			s.broadcast(ctx, prev.ProjectID)
		}
	}

	me := domain.VoiceParticipant{ConnID: connID, UserID: userID, Name: u.Name, Muted: muted}
	if err := s.rooms.Put(ctx, projectID, me, PresenceTTL); err != nil {
		return nil, err
	}
	return s.broadcast(ctx, projectID), nil
}

// Leave takes the connection out of the room.
func (s *Service) Leave(ctx context.Context, userID int64, connID string, projectID int64) error {
	if err := s.rooms.Remove(ctx, projectID, connID); err != nil {
		return err
	}
	if err := s.rooms.Release(ctx, userID, connID); err != nil {
		return err
	}
	s.broadcast(ctx, projectID)
	return nil
}

// SetMuted updates the participant's mute state for everyone.
func (s *Service) SetMuted(ctx context.Context, u domain.User, connID string, projectID int64, muted bool) error {
	if err := s.Heartbeat(ctx, u, connID, projectID, muted); err != nil {
		return err
	}
	s.broadcast(ctx, projectID)
	return nil
}

// Heartbeat keeps the participant listed.
func (s *Service) Heartbeat(ctx context.Context, u domain.User, connID string, projectID int64, muted bool) error {
	if err := s.rooms.Put(ctx, projectID, domain.VoiceParticipant{ConnID: connID, UserID: u.ID, Name: u.Name, Muted: muted}, PresenceTTL); err != nil {
		return err
	}
	_, err := s.rooms.Claim(ctx, u.ID, Slot{ProjectID: projectID, ConnID: connID}, PresenceTTL)
	return err
}

// Signal relays a WebRTC signaling message to another participant of the same room.
func (s *Service) Signal(ctx context.Context, userID int64, fromConnID string, projectID int64, toConnID string, data json.RawMessage) error {
	room, err := s.rooms.List(ctx, projectID)
	if err != nil {
		return err
	}
	in := func(id string) bool {
		return slices.ContainsFunc(room, func(p domain.VoiceParticipant) bool { return p.ConnID == id })
	}
	if !in(fromConnID) || !in(toConnID) {
		return domain.ErrVoiceNotJoined
	}
	return s.notify(ctx, toConnID, map[string]any{"type": "voice.signal", "from": fromConnID, "userId": userID, "signal": data})
}

// Rooms returns the non-empty voice rooms of the user's projects.
func (s *Service) Rooms(ctx context.Context, userID int64) (map[int64][]domain.VoiceParticipant, error) {
	projects, err := s.projects.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := map[int64][]domain.VoiceParticipant{}
	for _, p := range projects {
		room, err := s.rooms.List(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if len(room) > 0 {
			out[p.ID] = room
		}
	}
	return out, nil
}

// broadcast tells every member of the project who is in its room now, and returns that list.
// Failures only delay the update until the next change, so they are not returned.
func (s *Service) broadcast(ctx context.Context, projectID int64) []domain.VoiceParticipant {
	room, err := s.rooms.List(ctx, projectID)
	if err != nil {
		return nil
	}
	sortParticipants(room)
	members, err := s.list.List(ctx, projectID)
	if err != nil {
		return room
	}
	for _, m := range members {
		s.events.PublishUser(ctx, domain.UserEvent{Type: domain.UserVoiceChanged, UserID: m.ID, ProjectID: projectID, Voice: room})
	}
	return room
}

func (s *Service) notify(ctx context.Context, connID string, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.signals.PublishSignal(ctx, connID, b)
}

// sortParticipants keeps a stable order (by user, then connection), so lists don't jump around.
func sortParticipants(ps []domain.VoiceParticipant) {
	slices.SortFunc(ps, func(a, b domain.VoiceParticipant) int {
		return cmp.Or(cmp.Compare(a.UserID, b.UserID), cmp.Compare(a.ConnID, b.ConnID))
	})
}

// ---------- ICE servers ----------

type iceConfig struct {
	stun, turn []string
	secret     string
	ttl        time.Duration
}

// ICEServer is the browser's RTCIceServer.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEServers returns the STUN/TURN servers for a user. TURN credentials are short-lived, in the format
// of coturn's "use-auth-secret" (TURN REST API): username "<expiry>:<userID>", password
// base64(HMAC-SHA1(secret, username)), so no per-user state is kept on the TURN server.
func (s *Service) ICEServers(userID int64, now time.Time) []ICEServer {
	out := []ICEServer{}
	if len(s.ice.stun) > 0 {
		out = append(out, ICEServer{URLs: s.ice.stun})
	}
	if len(s.ice.turn) > 0 && s.ice.secret != "" {
		username := strconv.FormatInt(now.Add(s.ice.ttl).Unix(), 10) + ":" + strconv.FormatInt(userID, 10)
		mac := hmac.New(sha1.New, []byte(s.ice.secret))
		mac.Write([]byte(username))
		out = append(out, ICEServer{URLs: s.ice.turn, Username: username, Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil))})
	}
	return out
}
