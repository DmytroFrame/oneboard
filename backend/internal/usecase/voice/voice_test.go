package voice

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ---------- fakes ----------

type fakeRooms struct {
	rooms map[int64]map[string]domain.VoiceParticipant
	slots map[int64]Slot
}

func (f *fakeRooms) Put(_ context.Context, projectID int64, p domain.VoiceParticipant, _ time.Duration) error {
	if f.rooms[projectID] == nil {
		f.rooms[projectID] = map[string]domain.VoiceParticipant{}
	}
	f.rooms[projectID][p.ConnID] = p
	return nil
}

func (f *fakeRooms) Remove(_ context.Context, projectID int64, connID string) error {
	delete(f.rooms[projectID], connID)
	return nil
}

func (f *fakeRooms) List(_ context.Context, projectID int64) ([]domain.VoiceParticipant, error) {
	var out []domain.VoiceParticipant
	for _, p := range f.rooms[projectID] {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRooms) Claim(_ context.Context, userID int64, slot Slot, _ time.Duration) (*Slot, error) {
	old, ok := f.slots[userID]
	f.slots[userID] = slot
	if !ok {
		return nil, nil
	}
	return &old, nil
}

func (f *fakeRooms) Release(_ context.Context, userID int64, connID string) error {
	if f.slots[userID].ConnID == connID {
		delete(f.slots, userID)
	}
	return nil
}

type sent struct {
	to  string
	msg map[string]any
}

type fakeSignals struct{ out []sent }

func (f *fakeSignals) PublishSignal(_ context.Context, to string, payload []byte) error {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	f.out = append(f.out, sent{to, m})
	return nil
}

type fakeEvents struct{ out []domain.UserEvent }

func (f *fakeEvents) PublishUser(_ context.Context, e domain.UserEvent) { f.out = append(f.out, e) }

// project 1 has users 1..10, project 2 has users 1 and 2; user 99 is in neither.
type fakeMembers struct{}

func (fakeMembers) members(projectID int64) []int64 {
	switch projectID {
	case 1:
		return []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	case 2:
		return []int64{1, 2}
	}
	return nil
}

func (f fakeMembers) Role(_ context.Context, projectID, userID int64) (domain.Role, error) {
	for _, id := range f.members(projectID) {
		if id == userID {
			return domain.RoleMember, nil
		}
	}
	return domain.RoleNone, nil
}

func (f fakeMembers) List(_ context.Context, projectID int64) ([]domain.Member, error) {
	var out []domain.Member
	for _, id := range f.members(projectID) {
		out = append(out, domain.Member{User: domain.User{ID: id}})
	}
	return out, nil
}

func (f fakeMembers) ListForUser(_ context.Context, userID int64) ([]domain.Project, error) {
	var out []domain.Project
	for _, pid := range []int64{1, 2} {
		if r, _ := f.Role(context.Background(), pid, userID); r != domain.RoleNone {
			out = append(out, domain.Project{ID: pid})
		}
	}
	return out, nil
}

func newTestService() (*Service, *fakeRooms, *fakeSignals, *fakeEvents) {
	rooms := &fakeRooms{rooms: map[int64]map[string]domain.VoiceParticipant{}, slots: map[int64]Slot{}}
	signals, events := &fakeSignals{}, &fakeEvents{}
	s := &Service{rooms: rooms, signals: signals, events: events, members: fakeMembers{}, list: fakeMembers{}, projects: fakeMembers{}}
	return s, rooms, signals, events
}

// ---------- tests ----------

var ctx = context.Background()

func TestJoinTellsEveryMember(t *testing.T) {
	s, _, _, events := newTestService()
	got, err := s.Join(ctx, domain.User{ID: 1, Name: "u"}, "a", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ConnID != "a" {
		t.Fatalf("participants = %+v", got)
	}
	if len(events.out) != 2 {
		t.Fatalf("want an event for each of the 2 members, got %d", len(events.out))
	}
	for _, e := range events.out {
		if e.Type != domain.UserVoiceChanged || e.ProjectID != 2 || len(e.Voice) != 1 {
			t.Fatalf("event = %+v", e)
		}
	}
}

func TestJoinRequiresMembership(t *testing.T) {
	s, _, _, _ := newTestService()
	if _, err := s.Join(ctx, domain.User{ID: 99, Name: "u"}, "x", 1, false); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v, want project_not_found", err)
	}
}

func TestRoomLimit(t *testing.T) {
	s, _, _, _ := newTestService()
	for i := int64(1); i <= domain.VoiceRoomLimit; i++ {
		if _, err := s.Join(ctx, domain.User{ID: i, Name: "u"}, fmt.Sprint("c", i), 1, false); err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
	}
	if _, err := s.Join(ctx, domain.User{ID: 9, Name: "u"}, "c9", 1, false); !errors.Is(err, domain.ErrVoiceRoomFull) {
		t.Fatalf("err = %v, want voice_room_full", err)
	}
	// Someone already inside can rejoin (e.g. after a reconnect) even when the room is full.
	if _, err := s.Join(ctx, domain.User{ID: 1, Name: "u"}, "c1b", 1, false); err != nil {
		t.Fatalf("rejoin: %v", err)
	}
}

func TestJoinFromAnotherTabReplacesTheFirst(t *testing.T) {
	s, rooms, signals, _ := newTestService()
	if _, err := s.Join(ctx, domain.User{ID: 1, Name: "u"}, "tab1", 1, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join(ctx, domain.User{ID: 1, Name: "u"}, "tab2", 2, false); err != nil {
		t.Fatal(err)
	}
	if len(rooms.rooms[1]) != 0 {
		t.Fatalf("the first tab is still in room 1: %+v", rooms.rooms[1])
	}
	if len(signals.out) != 1 || signals.out[0].to != "tab1" || signals.out[0].msg["type"] != "voice.left" || signals.out[0].msg["reason"] != "elsewhere" {
		t.Fatalf("signals = %+v", signals.out)
	}
}

func TestSignalOnlyWithinTheRoom(t *testing.T) {
	s, _, signals, _ := newTestService()
	_, _ = s.Join(ctx, domain.User{ID: 1, Name: "u"}, "a", 1, false)
	_, _ = s.Join(ctx, domain.User{ID: 2, Name: "u"}, "b", 1, false)
	_, _ = s.Join(ctx, domain.User{ID: 3, Name: "u"}, "c", 2, false) // another room — user 3 isn't in project 2, so this fails

	if err := s.Signal(ctx, 1, "a", 1, "b", json.RawMessage(`{"sdp":"x"}`)); err != nil {
		t.Fatal(err)
	}
	last := signals.out[len(signals.out)-1]
	if last.to != "b" || last.msg["from"] != "a" || last.msg["type"] != "voice.signal" {
		t.Fatalf("signal = %+v", last)
	}
	if err := s.Signal(ctx, 1, "a", 1, "nobody", json.RawMessage(`{}`)); !errors.Is(err, domain.ErrVoiceNotJoined) {
		t.Fatalf("err = %v, want voice_not_joined", err)
	}
}

func TestLeave(t *testing.T) {
	s, rooms, _, events := newTestService()
	_, _ = s.Join(ctx, domain.User{ID: 1, Name: "u"}, "a", 2, false)
	events.out = nil
	if err := s.Leave(ctx, 1, "a", 2); err != nil {
		t.Fatal(err)
	}
	if len(rooms.rooms[2]) != 0 || len(rooms.slots) != 0 {
		t.Fatal("still in the room")
	}
	if len(events.out) != 2 || len(events.out[0].Voice) != 0 {
		t.Fatalf("events = %+v", events.out)
	}
}

func TestRoomsListsOnlyNonEmpty(t *testing.T) {
	s, _, _, _ := newTestService()
	_, _ = s.Join(ctx, domain.User{ID: 2, Name: "u"}, "b", 2, true)
	rooms, err := s.Rooms(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 || len(rooms[2]) != 1 || !rooms[2][0].Muted {
		t.Fatalf("rooms = %+v", rooms)
	}
}

func TestTURNCredentials(t *testing.T) {
	s, _, _, _ := newTestService()
	if got := s.ICEServers(7, time.Now()); len(got) != 0 {
		t.Fatalf("nothing configured, got %+v", got)
	}
	s.ice = iceConfig{stun: []string{"stun:example.org"}, turn: []string{"turn:example.org:3478"}, secret: "s3cret", ttl: time.Hour}
	now := time.Unix(1_000_000, 0)
	got := s.ICEServers(7, now)
	if len(got) != 2 {
		t.Fatalf("servers = %+v", got)
	}
	turn := got[1]
	if turn.Username != "1003600:7" {
		t.Fatalf("username = %q", turn.Username)
	}
	mac := hmac.New(sha1.New, []byte("s3cret"))
	mac.Write([]byte(turn.Username))
	if turn.Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) || !strings.HasPrefix(turn.URLs[0], "turn:") {
		t.Fatalf("credential = %+v", turn)
	}
}
