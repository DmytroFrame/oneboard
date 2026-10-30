// Package collab implements real-time collaboration on a board: who is here, what they have open,
// relaying cursors/drags between them, and the shared (CRDT) documents of the cards they edit.
// It is transport-agnostic: the WebSocket hub in the HTTP layer drives it.
package collab

import (
	"context"
	"errors"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = fx.Module("collab", fx.Provide(NewService))

// PresenceTTL is how long a peer stays listed without a heartbeat (e.g. after a server crash).
const PresenceTTL = 60 * time.Second

// Message is something published to a board's channel: either a committed board event or a raw
// peer message (cursor, drag, document update…) to forward to the other connections.
type Message struct {
	ProjectID int64
	From      string             // sender's connection id ("" for events)
	Event     *domain.BoardEvent // set for events
	Payload   []byte             // set for peer messages
	UserEvent *domain.UserEvent  // set for user events (ProjectID is 0)
	SignalTo  string             // set for a message to one connection (Payload, already encoded)
}

// Broker is the pub/sub between server instances.
type Broker interface {
	PublishPeer(ctx context.Context, projectID int64, from string, payload []byte) error
	// Subscribe delivers every board's messages until ctx ends.
	Subscribe(ctx context.Context) (<-chan Message, error)
}

type Presence interface {
	Put(ctx context.Context, projectID int64, p domain.Peer, ttl time.Duration) error
	Remove(ctx context.Context, projectID int64, connID string) error
	List(ctx context.Context, projectID int64) ([]domain.Peer, error)
}

// DocLog keeps the Yjs updates of card documents while they are being edited.
type DocLog interface {
	Load(ctx context.Context, cardID int64) (base int64, updates [][]byte, err error)
	// ClaimSeed lets exactly one client initialise an empty document from the database.
	ClaimSeed(ctx context.Context, cardID int64, connID string) (bool, error)
	// Append stores an update and returns its sequence number.
	Append(ctx context.Context, cardID int64, update []byte) (int64, error)
	// Compact replaces all entries up to seq with a snapshot of the document state.
	Compact(ctx context.Context, cardID, upto int64, state []byte) error
}

// Authenticator resolves an access token (auth.Service implements it).
type Authenticator interface {
	Authenticate(ctx context.Context, accessToken string) (domain.User, error)
}

type Cards interface {
	Get(ctx context.Context, cardID int64) (domain.Card, error)
}

type Service struct {
	broker   Broker
	presence Presence
	docs     DocLog
	auth     Authenticator
	members  usecase.Memberships
	cards    Cards
}

type Deps struct {
	fx.In
	Broker   Broker
	Presence Presence
	Docs     DocLog
	Auth     Authenticator
	Members  usecase.Memberships
	Cards    Cards
}

func NewService(d Deps) *Service {
	return &Service{d.Broker, d.Presence, d.Docs, d.Auth, d.Members, d.Cards}
}

// Session is one authenticated connection: to a board, or (ProjectID 0) the user's own channel,
// which every signed-in tab keeps open to hear about changes to its access.
type Session struct {
	ConnID    string
	User      domain.User
	ProjectID int64
}

func (s Session) UserOnly() bool { return s.ProjectID == 0 }

// Join authenticates the connection, checks access to the board, announces the peer and
// returns everyone already there.
func (s *Service) Join(ctx context.Context, token string, projectID int64, connID string) (Session, []domain.Peer, error) {
	u, err := s.auth.Authenticate(ctx, token)
	if err != nil {
		return Session{}, nil, err
	}
	sess := Session{ConnID: connID, User: u, ProjectID: projectID}
	if sess.UserOnly() {
		return sess, nil, nil
	}
	if _, err := usecase.RequireMember(ctx, s.members, projectID, u.ID, domain.ErrProjectNotFound); err != nil {
		return Session{}, nil, err
	}
	peers, err := s.presence.List(ctx, projectID)
	if err != nil {
		return Session{}, nil, err
	}
	me := domain.Peer{ConnID: connID, UserID: u.ID}
	if err := s.presence.Put(ctx, projectID, me, PresenceTTL); err != nil {
		return Session{}, nil, err
	}
	return sess, peers, s.Relay(ctx, sess, peerMessage{Type: "join", Peer: &me})
}

// Leave removes the peer and tells the others.
func (s *Service) Leave(ctx context.Context, sess Session) error {
	if sess.UserOnly() {
		return nil
	}
	if err := s.presence.Remove(ctx, sess.ProjectID, sess.ConnID); err != nil {
		return err
	}
	return s.Relay(ctx, sess, peerMessage{Type: "leave", ConnID: sess.ConnID})
}

// Heartbeat keeps the peer listed; focus is re-sent so it survives a presence expiry.
func (s *Service) Heartbeat(ctx context.Context, sess Session, focus *int64) error {
	if sess.UserOnly() {
		return nil
	}
	return s.presence.Put(ctx, sess.ProjectID, domain.Peer{ConnID: sess.ConnID, UserID: sess.User.ID, CardID: focus}, PresenceTTL)
}

// CheckCard verifies that a card belongs to the session's board.
func (s *Service) CheckCard(ctx context.Context, sess Session, cardID int64) error {
	c, err := s.cards.Get(ctx, cardID)
	if errors.Is(err, domain.ErrNoRows) || (err == nil && c.ProjectID != sess.ProjectID) {
		return domain.ErrCardNotFound
	}
	return err
}

// Focus records which card the peer has open (nil = none) and tells the others.
// The caller has checked the card with CheckCard.
func (s *Service) Focus(ctx context.Context, sess Session, cardID *int64) error {
	if err := s.Heartbeat(ctx, sess, cardID); err != nil {
		return err
	}
	return s.Relay(ctx, sess, peerMessage{Type: "focus", ConnID: sess.ConnID, CardID: cardID})
}

// OpenDoc returns the card document's history. The caller has checked the card with CheckCard.
func (s *Service) OpenDoc(ctx context.Context, sess Session, cardID int64) (domain.DocInit, error) {
	base, updates, err := s.docs.Load(ctx, cardID)
	if err != nil {
		return domain.DocInit{}, err
	}
	init := domain.DocInit{Base: base, Updates: updates}
	if base == 0 && len(updates) == 0 {
		if init.Seed, err = s.docs.ClaimSeed(ctx, cardID, sess.ConnID); err != nil {
			return domain.DocInit{}, err
		}
	}
	return init, nil
}

// AppendDoc stores a document update and returns its sequence number; relaying it is up to the caller,
// which already has the client's message.
func (s *Service) AppendDoc(ctx context.Context, cardID int64, update []byte) (int64, error) {
	return s.docs.Append(ctx, cardID, update)
}

func (s *Service) CompactDoc(ctx context.Context, cardID, upto int64, state []byte) error {
	return s.docs.Compact(ctx, cardID, upto, state)
}

// Relay forwards a peer message to everyone else on the board (on every server instance).
func (s *Service) Relay(ctx context.Context, sess Session, msg any) error {
	payload, err := marshal(msg)
	if err != nil {
		return err
	}
	return s.broker.PublishPeer(ctx, sess.ProjectID, sess.ConnID, payload)
}

// RelayRaw forwards an already encoded message.
func (s *Service) RelayRaw(ctx context.Context, sess Session, payload []byte) error {
	return s.broker.PublishPeer(ctx, sess.ProjectID, sess.ConnID, payload)
}

func (s *Service) Subscribe(ctx context.Context) (<-chan Message, error) {
	return s.broker.Subscribe(ctx)
}
