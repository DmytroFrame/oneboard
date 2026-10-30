package domain

// BoardEventType names a change on a board that everyone looking at it should see.
type BoardEventType string

const (
	EventCardCreated    BoardEventType = "card.created"
	EventCardUpdated    BoardEventType = "card.updated"
	EventCardMoved      BoardEventType = "card.moved"
	EventCardDeleted    BoardEventType = "card.deleted"
	EventColumnsChanged BoardEventType = "columns.changed"
	EventMembersChanged BoardEventType = "members.changed"
	EventProjectDeleted BoardEventType = "project.deleted"
)

// BoardEvent is published after a change is committed. Exported fields only: it travels between
// server instances as JSON.
type BoardEvent struct {
	Type         BoardEventType
	ProjectID    int64
	ActorID      int64
	Origin       string // real-time connection (browser tab) that caused it, if any
	Card         *Card  // created, updated, moved
	CardID       int64  // deleted
	ColumnID     int64  // deleted: the column it was in
	FromColumnID int64  // moved
	AfterID      *int64 // moved: neighbours in the target column, so clients can place it
	BeforeID     *int64
	// RemovedUserID (members.changed) lost access: their open connections to the board are closed.
	RemovedUserID int64
}

// UserEventType names a change that concerns one user wherever they are in the app.
type UserEventType string

const (
	// UserAccessChanged: the user's own access changed (see AccessReason).
	UserAccessChanged UserEventType = "access"
	// UserOrgMembersChanged: the member list of one of the user's organizations changed.
	UserOrgMembersChanged UserEventType = "org.members"
)

type AccessReason string

const (
	AccessAdded   AccessReason = "added"
	AccessRemoved AccessReason = "removed"
	AccessRole    AccessReason = "role"    // their role changed
	AccessRenamed AccessReason = "renamed" // the organization/project was renamed
	AccessDeleted AccessReason = "deleted" // the organization/project was deleted
)

// UserEvent is delivered to one user's open tabs.
type UserEvent struct {
	Type      UserEventType
	UserID    int64
	Reason    AccessReason
	OrgID     int64
	ProjectID int64
	Name      string // the organization's or project's name, for notifications
	ActorID   int64
}

// Peer is one open board tab (a user may have several).
type Peer struct {
	ConnID string `json:"connId"`
	UserID int64  `json:"userId"`
	CardID *int64 `json:"cardId"` // the task they have open, if any
}

// DocInit is what a client gets when it opens a card's collaborative document.
type DocInit struct {
	Base    int64    // sequence number of the entry before Updates[0]
	Updates [][]byte // Yjs updates (the first may be a compacted snapshot)
	Seed    bool     // nobody has the document yet: this client must load it from the card
}
