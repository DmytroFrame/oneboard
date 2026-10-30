package collab

import (
	"encoding/json"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// peerMessage is the subset of the wire protocol the service produces itself (join/leave/focus);
// cursor, drag and document messages are relayed as the transport encoded them.
type peerMessage struct {
	Type   string       `json:"type"`
	Peer   *domain.Peer `json:"peer,omitempty"`
	ConnID string       `json:"connId,omitempty"`
	CardID *int64       `json:"cardId"`
}

func marshal(v any) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	return json.Marshal(v)
}
