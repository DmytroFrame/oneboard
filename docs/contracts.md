# Inter-service contracts

What crosses a process boundary besides the public REST API (README → REST API) and the browser's
WebSocket protocol (README → Real time). Today these are messages between instances of the one backend
over Redis; when the backend is split (see [architecture.md](architecture.md)) they are what services
exchange, in whatever language they are written.

The Go definitions are in `backend/internal/contract`. Names are camelCase; changing or removing a field
is a breaking change for every consumer, adding an optional one is not. `contract/events_test.go` pins
the wire names and checks that messages from the previous release still decode (rolling deploys).

## Pub/sub channels (Redis)

Every instance pattern-subscribes to all three prefixes and delivers to the sockets it holds.

### `board:<projectId>`: board events and peer messages

```json
{ "event": { …BoardEvent… } }
{ "from": "<connId>", "payload": { …peer message… } }
```

`payload` is a browser message (cursor, drag, focus, join/leave, doc.update, awareness) relayed as is and
not delivered back to `from`. `event` is a committed change:

| Field | Type | |
|---|---|---|
| `type` | string | `card.created`, `card.updated`, `card.moved`, `card.deleted`, `columns.changed`, `members.changed`, `project.deleted` |
| `projectId` | int | |
| `actorId` | int | user who made the change |
| `origin` | string? | connection id of the browser tab that made it; that tab ignores the event |
| `card` | Card? | `card.created`, `card.updated`, `card.moved` |
| `cardId`, `columnId` | int? | `card.deleted`: the card and the column it was in |
| `fromColumnId`, `afterId`, `beforeId` | int? | `card.moved`: source column and the card's new neighbours |
| `removedUserId` | int? | `members.changed`: this user lost access; their board sockets are closed |

**Card:** `id`, `projectId`, `columnId`, `key` (`KAN-12`), `title`, `description` (markdown), `priority`
(`low`/`medium`/`high`/`critical`), `position`, `authorId`, `assigneeId` (null = unassigned), `createdAt`,
`updatedAt` (RFC 3339), `attachmentCount`, `cover` (`{attachmentId, kind: "image"|"video", hasThumb}` or
null).

```json
{
  "event": {
    "type": "card.moved", "projectId": 3, "actorId": 5, "origin": "9f2c1a0b7d3e4f56",
    "fromColumnId": 9, "afterId": 41,
    "card": {
      "id": 42, "projectId": 3, "columnId": 10, "key": "KAN-42", "title": "Landing page",
      "description": "", "priority": "high", "position": 2, "authorId": 5, "assigneeId": 7,
      "createdAt": "2026-11-02T10:00:00Z", "updatedAt": "2026-11-02T10:05:00Z",
      "attachmentCount": 1, "cover": { "attachmentId": 8, "kind": "image", "hasThumb": true }
    }
  }
}
```

### `user:<userId>`: events for one user

| Field | Type | |
|---|---|---|
| `type` | string | `access` (their own access changed), `org.members` (an organization's member list changed), `voice` (a voice room changed) |
| `userId` | int | recipient |
| `reason` | string? | `access`: `added`, `removed`, `role`, `renamed`, `deleted` |
| `orgId`, `projectId` | int? | what it is about |
| `name` | string? | the organization's or project's name, for notifications |
| `actorId` | int? | |
| `voice` | VoiceParticipant[]? | `voice`: everyone now in the project's room — `{connId, userId, name, muted}` |

### `rtc:<connId>`: messages for one connection

Opaque JSON for one browser tab, published by the voice use case: `voice.signal {from, userId, signal}`
(WebRTC SDP or ICE, never parsed) and `voice.left {projectId, reason}`.

## Shared state (Redis)

Owned by the Realtime context. Another implementation must use the same layout, or the instances it runs
next to won't see each other's users. Every structure works on Redis/Valkey without 7.4-only commands
(ElastiCache, cluster mode disabled: the Lua scripts compute keys).

| Key | Type | Content |
|---|---|---|
| `presence:<projectId>` | hash | connection id → `{connId, userId, cardId}` |
| `presence:<projectId>:exp` | sorted set | connection id → expiry (Unix ms); expired entries are removed on read |
| `voice:<projectId>`, `voice:<projectId>:exp` | hash, sorted set | same as presence, with VoiceParticipant values |
| `voice:user:<userId>` | string | `<projectId>:<connId>`: the user's one current room |
| `ydoc:<cardId>` | list | base64 Yjs updates; the first may be a compacted snapshot |
| `ydoc:<cardId>:base` | string | sequence number of the entry before the list's first |
| `ydoc:<cardId>:seed` | string | connection id that may seed the document from Postgres (`SET NX`, 15 s) |

Identity's refresh tokens (`rt:*`, `fam:*`, `famprev:*`) and access-token denylist (`revoked:*`) are
private to it and not part of this contract.
