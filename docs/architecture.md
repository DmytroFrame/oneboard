# Architecture

OneBoard is a **modular monolith**: one Go binary and one React SPA, organized so that the backend can
later be split into services (in Go or other languages) without rewriting the business logic. This page
describes the boundaries, the rules that keep them clean, the couplings that still exist, and how to
extract a service when the time comes.

- [Backend](#backend): layers, modules, contexts
- [Splitting into services](#splitting-into-services)
- [Frontend](#frontend): folders, components, styles

## Backend

### Layers

```
transport/httpapi  →  usecase/<module>  →  domain
                           ↑ ports (interfaces)
                      adapter/<technology>
```

- `domain`: entities, business errors (`catalog.go`), event types. No dependencies, no JSON tags.
- `usecase/<module>`: business logic and permission checks. Each package declares the interfaces
  (ports) it needs; it never imports another use case package.
- `adapter/<technology>`: Postgres, Redis, S3, OpenSearch… implement the ports and bind themselves to
  them in their fx module.
- `transport/httpapi`: Echo handlers and the WebSocket hub. Each module's transport is
  `<module>.go` (routes + handlers) plus `<module>_dto.go` (its JSON shapes and Presenter methods).
- `contract`: messages that cross a process boundary (see [contracts.md](contracts.md)).

### Bounded contexts

The use case packages group into five contexts. Each owns its data; nothing outside a context writes to it
except through that context's use cases.

| Context | Use cases | Owns (Postgres) | Owns (elsewhere) | Publishes |
|---|---|---|---|---|
| **Identity** | `auth`, `user` | `users` | Redis `rt:*`, `fam:*`, `famprev:*` (refresh tokens), `revoked:*` (access-token denylist); S3 `avatars/` | — |
| **Workspace** | `organization`, `project` | `organizations`, `organization_members`, `organization_invitations`, `projects`, `project_members` | — | `members.changed`, `project.deleted`, user events `access`, `org.members` |
| **Board** | `board` (columns, cards), `attachment` | `board_columns`, `cards`, `card_attachments` | OpenSearch index; S3 `attachments/` | `card.*`, `columns.changed` |
| **Realtime** | `collab`, `voice` | — | Redis `presence:*`, `voice:*`, `ydoc:*`, pub/sub channels | user events `voice` |
| **Dashboard** | `dashboard` | — (read model) | — | — |

Shared ports (`usecase/usecase.go`): `Transactor`, `Memberships` (a user's role in a project, owned by
Workspace and read by Board, Realtime and Attachments), `BoardEvents`, `UserEvents`, `WithOrigin`.

### Rules that keep it splittable

1. **No use case imports another.** Cross-context needs go through a port, which an adapter (today) or
   a client of another service (tomorrow) implements. The one service-to-service binding, `auth.Service`
   → `collab.Authenticator`, is a port too.
2. **Events are contracts.** What use cases publish leaves the process as `contract.*` JSON with stable
   camelCase names, never as Go structs. A consumer in any language can read it from
   [contracts.md](contracts.md).
3. **Transport per module.** A module's routes, handlers and DTOs sit in its own files, so moving it to
   another binary is a file move.
4. **Domain per context.** `domain/users.go`, `projects.go`, `organizations.go`, `board.go`,
   `attachments.go`, `realtime.go`.
5. **Stateless instances.** Sessions, presence and documents live in Redis; any instance serves any
   request or socket. Several instances can start at once (migrations take an advisory lock).

### Couplings that still exist

These are fine in one process, and are what an extraction has to replace:

- **Cross-context SQL.** Repositories join across contexts for convenience: cards ⋈ projects (task key
  prefix), projects ⋈ organizations (organization name), member and card counts, and the dashboard,
  which reads everything. A service would own its tables and keep copies of what it shows
  (e.g. Board stores the project key it got from a `project.created` event).
- **Cross-context transactions.** Removing someone from an organization also removes them from its
  projects and unassigns their cards in one transaction; deleting a project or organization deletes cards,
  files and index entries. Across services these become events (`org.member_removed`, `project.deleted`)
  that Board handles on its own.
- **`Memberships` is synchronous.** Board, Attachments and Realtime ask Workspace for a role on every
  request. Across services: a cached gRPC/HTTP call, or a membership table replicated from Workspace
  events.

### Splitting into services

The order that costs least:

1. **Realtime** (`collab`, `voice`, the hub). It has no tables, already talks to the rest only through
   Redis and ports, and is the part that scales differently (long-lived sockets). New binary
   `cmd/realtime` linking `usecase/collab`, `usecase/voice`, `adapter/redisstore` and the hub; it verifies
   JWTs locally (shared `JWT_SECRET` or a public key) and gets `Memberships` from Workspace. The API
   service keeps publishing events to the same Redis channels.
2. **Durable events.** Redis pub/sub is fire-and-forget, which is enough for UI updates but not for
   services that must not miss a `project.deleted`. Add an outbox table written in the same transaction
   as the change, relayed to Kafka (Redpanda is already in `compose.yaml`); keep the `contract` types as
   the message schema.
3. **Board**, then **Workspace**, then **Identity**, each taking its tables (a database or schema per
   service) and replacing the joins and transactions listed above with its own copies and event handlers.
4. **Dashboard** becomes a read model fed by events, or a backend-for-frontend calling the others.

A service in another language implements the same HTTP API (README), WebSocket protocol (README → Real
time) and messages (contracts.md); the error codes in `domain/catalog.go` are part of the API.

## Frontend

React + TypeScript, organized by type. Every component and page is a folder with its `.tsx` and a CSS
module next to it.

```
frontend/src/
  main.tsx, App.tsx       entry (global styles, i18n, PWA) and routes
  components/
    ui/                   design-system primitives: Button, IconButton, Modal, Field, Form, Panel,
                          Menu, Banner, Badge, Spinner, Muted, icons.tsx…
    <Name>/               app components: Name.tsx + Name.module.scss (+ sub-parts sharing the module,
                          e.g. CardModal/NewCard.tsx, MemberList/MemberRow.tsx)
  layouts/                AppLayout (signed-in shell + useLayout context), AuthLayout
  pages/<Name>/           routes: Name.tsx + Name.module.scss
  hooks/                  useBoardRealtime, useDebounced, useClickOutside, useEscape…
  context/                auth (session, route guards), preferences (theme, language)
  lib/                    api client, realtime socket, cardDoc (Yjs), voice (WebRTC), pwa, touch, cx…
  i18n/                   en.ts (source of truth), uk.ts, de.ts
  types/                  API types
  styles/                 _tokens.scss, _mixins.scss, global.scss
```

### Styles

- **CSS modules + Sass.** `import s from "./Card.module.scss"`, `className={s.card}`; class names are
  camelCase and scoped to the file. Combine with `cx(s.card, active && s.active)` (`lib/cx.ts`).
- **Tokens** are CSS custom properties in `styles/_tokens.scss` (light, and dark under
  `:root[data-theme="dark"]`). Use them (`var(--glass)`, `var(--accent)`, `var(--success)`…), never raw
  colours.
- **Mixins** in `styles/_mixins.scss` are injected into every module by `vite.config.ts`: breakpoints
  (`@include mobile`, `touch`, `standalone`, `reduced-motion`, `dark`) and surfaces (`glass`,
  `glass-panel`, `glass-menu`, `glass-tile`, `ellipsis`, `eyebrow`…). Responsive rules live next to the
  rules they change, not in a separate mobile section.
- **`styles/global.scss`** is the only global stylesheet: tokens, element defaults (body, inputs) and
  hooks for third-party markup (the touch-drag image). TipTap's own class names are styled with
  `:global(...)` inside `DescriptionEditor.module.scss`.
- **Overriding a child component:** pass `className` and nest the selector under the parent's own class
  (`.peek .peekAvatar`), so it wins regardless of stylesheet order.
- **Links that look like buttons:** `buttonClass({ variant, size })` / `iconButtonClass()`.
- **DOM hooks for non-React code** use data attributes, not classes: `data-card-id`, `data-column-id`
  (cursor anchors, drop targets), `data-board-columns`, `data-column-body` (touch auto-scroll),
  `data-slash-menu` (Escape handling).

### Conventions

- One exported component per file; a page is the default export of its folder's file.
- Import with the `@/` alias (`@/components/ui/Button/Button`); no barrel `index.ts` files, so the lazy
  `CardModal` chunk (TipTap + Yjs) stays out of the main bundle.
- Logic that isn't UI goes to `lib/` (plain modules) or `hooks/` (React hooks).
