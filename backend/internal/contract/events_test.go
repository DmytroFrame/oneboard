package contract

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

func sampleBoardEvent() domain.BoardEvent {
	assignee, after := int64(7), int64(41)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	return domain.BoardEvent{
		Type: domain.EventCardMoved, ProjectID: 3, ActorID: 5, Origin: "abc123", FromColumnID: 9, AfterID: &after,
		Card: &domain.Card{
			ID: 42, ProjectID: 3, ColumnID: 10, Key: "KAN-42", Title: "Title", Description: "Text",
			Priority: domain.PriorityHigh, Position: 2, AuthorID: 5, AssigneeID: &assignee,
			CreatedAt: now, UpdatedAt: now, AttachmentCount: 1,
			Cover: &domain.CardCover{AttachmentID: 8, Kind: domain.AttachmentImage, HasThumb: true},
		},
	}
}

func TestBoardEventRoundTrip(t *testing.T) {
	in := sampleBoardEvent()
	data, err := json.Marshal(NewBoardEvent(in))
	if err != nil {
		t.Fatal(err)
	}
	var out BoardEvent
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.Domain(); !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip changed the event:\n got %+v\nwant %+v", got, in)
	}
}

// Instances deployed before this package published domain structs without JSON tags.
// A rolling deploy mixes both encodings, so the old one must still decode.
func TestDecodesPreviousEncoding(t *testing.T) {
	in := sampleBoardEvent()
	legacy, _ := json.Marshal(struct {
		Event *domain.BoardEvent `json:"event"`
	}{&in})
	var env BoardEnvelope
	if err := json.Unmarshal(legacy, &env); err != nil {
		t.Fatal(err)
	}
	if got := env.Event.Domain(); !reflect.DeepEqual(got, in) {
		t.Fatalf("legacy board event:\n got %+v\nwant %+v", got, in)
	}

	user := domain.UserEvent{
		Type: domain.UserVoiceChanged, UserID: 2, ProjectID: 3, Name: "Board", ActorID: 4,
		Voice: []domain.VoiceParticipant{{ConnID: "c1", UserID: 4, Name: "Ann", Muted: true}},
	}
	legacy, _ = json.Marshal(user)
	var ue UserEvent
	if err := json.Unmarshal(legacy, &ue); err != nil {
		t.Fatal(err)
	}
	if got := ue.Domain(); !reflect.DeepEqual(got, user) {
		t.Fatalf("legacy user event:\n got %+v\nwant %+v", got, user)
	}
}

func TestWireNamesAreCamelCase(t *testing.T) {
	data, _ := json.Marshal(NewBoardEvent(sampleBoardEvent()))
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	for _, key := range []string{"type", "projectId", "actorId", "origin", "card", "fromColumnId", "afterId"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in %s", key, data)
		}
	}
	if card := m["card"].(map[string]any); card["assigneeId"] == nil || card["cover"].(map[string]any)["attachmentId"] == nil {
		t.Errorf("card fields not camelCase: %s", data)
	}
}
