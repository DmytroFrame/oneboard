# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

OneBoard: a kanban app. Go backend (`backend/`) + React/TypeScript SPA (`frontend/`), backed by PostgreSQL, Redis, OpenSearch and RustFS (S3) from `compose.yaml`. `README.md` documents features, configuration, the full REST API and the real-time protocol. Repository: https://github.com/DmytroFrame/oneboard (Go module `github.com/DmytroFrame/oneboard/backend`). The product name is spelled **OneBoard**; technical identifiers (DB, bucket, index, Docker, storage keys) use `oneboard`.

## Commands

```bash
make infra-up        # docker compose up -d (Postgres, Redis, OpenSearch, RustFS, coturn, Redpanda); needs OrbStack/Docker running
make dev-backend     # go run ./cmd/server on :8080 (migrates + seeds an empty DB on start)
make dev-frontend    # Vite on :5173, proxies /api (incl. the /api/ws WebSocket) to :8080
make run             # build SPA + binary, serve everything from :8080
make test            # go vet + go test + frontend type-check/build
make docker-build    # production image (API + SPA), see docs/aws.md

cd backend && go build ./... && go vet ./... && go test ./...
cd backend && go test ./internal/adapter/jwttoken -run TestExpired   # single test
cd frontend && npm run build    # tsc --noEmit + vite build (this is the type check; there is no linter)
```

Config comes from env, falling back to the repo-root `.env` (shared with docker compose; `JWT_SECRET` is required). `APP_ENV` (`development` | `production`) switches the defaults in `internal/config/config.go`: production means AWS (RDS TLS verify-full, S3/OpenSearch via the default AWS credential chain + SigV4, ElastiCache TLS, JSON logs, no seed, secure cookies). Deployment notes: `docs/aws.md`, `.env.aws.example`. Demo login: `demo@example.com` / `demo1234`.

**Testing against live services without touching the user's data:** run a second server with an isolated DB/keyspace, e.g. `ADDR=:18090 REDIS_DB=1 S3_BUCKET=oneboard-test OPENSEARCH_INDEX=oneboard-test DATABASE_URL=postgres://…/oneboard_test` (create the DB first; drop it, `FLUSHDB` Redis db 1, delete the index/bucket afterwards). Yjs logs (`ydoc:<cardId>`) and presence live in Redis keyed by IDs, so sharing Redis db 0 with a test DB corrupts real cards. Port 18081 is taken by Redpanda's schema registry.

## Backend architecture (Clean Architecture + uber/fx)

Dependencies point inward: `transport/httpapi → usecase → domain`; `adapter/*` implement ports.

It is a modular monolith meant to be split into services later: bounded contexts (Identity, Workspace, Board, Realtime, Dashboard), the rules that keep them separable (no use case imports another; events cross processes only as `internal/contract` types; transport per module) and the extraction plan are in `docs/architecture.md`.

- `internal/domain` — entities, `Error{Kind, Code, Msg}`, and **`catalog.go`: every business error with a stable `code`**. The HTTP layer returns `{"error": <English msg>, "code": <code>}`; the frontend translates `errors.<code>`. Adding an error means adding the code to the catalog *and* to all three frontend locales.
- `internal/usecase/<module>` — business logic. Each package declares the interfaces (ports) it consumes; shared ports (`Transactor`, `Memberships`, `BoardEvents`, `UserEvents`, `RequireMember/RequireOwner`, `WithOrigin`) are in `usecase/usecase.go`. Permission checks live here (outsiders get NotFound, not Forbidden).
- **Modules register themselves** (`internal/modules`): every package declares `var Module = modules.Register(modules.<Namespace>, fx.Module(...))` in its own file (namespaces: `Core`, `Adapters`, `UseCases`, `Transport`, `Routes`). `internal/app` assembles everything with `modules.All()`; the only list is `internal/app/link.go` (blank imports — a new package must be added there or its registration never runs). `go test ./internal/app` validates the whole fx graph (`fx.ValidateApp`) without infrastructure — run it after changing wiring.
- `internal/adapter/*` — modules provide concrete types **and bind them to every port they satisfy** with one-line providers like `func(r *CardRepo) board.Cards { return r }` (see `adapter/postgres/db.go`). Use case services that implement another package's port bind themselves the same way (e.g. `auth` → `collab.Authenticator`). A missing binding fails `TestDependencyGraph`.
- `internal/transport/httpapi` — **Echo v5** (`*echo.Context`, handlers return `error`). Each handler file registers itself with `func init() { registerRoutes(NewXHandler) }` and implements `Mount(public, private *echo.Group)` (both under `/api`; private requires the bearer token). Path params are `:id`. Return `domain.Err…` or the transport's `apiError`s (`errInvalidJSON`, `errInvalidID`, `errNotFound`); `errorHandler` in `respond.go` renders `{"error","code"}`. Use `bind(c, &v)` / `pathID(c, "id")`; `currentUser(c)` reads the request context (where `WithOrigin` lives too). `NoGroupAutoRegister404Routes` is on, so unknown `/api` paths are 404 and wrong methods 405 (the SPA fallback refuses `/api/*`). Each module's JSON shapes and `Presenter` methods live in `<module>_dto.go` next to its handlers; `presenter.go` signs URLs (avatars, attachments) because `<img>`/`<video>` can't send the bearer token.
- `internal/contract` — messages that cross a process boundary (Redis pub/sub today, other services later): explicit camelCase JSON types + domain converters, documented in `docs/contracts.md`. Never publish domain structs; `contract/events_test.go` pins wire names and backwards compatibility.
- `internal/app/app.go` — `Options()` = logger + `modules.All()` + `bootstrap` (seeds demo data and syncs the search index before the server starts).

Key cross-cutting mechanics:
- **TLS:** `TLS_CERT_FILE` + `TLS_KEY_FILE` (both must exist) switch `StartServer` to HTTPS on `HTTPS_ADDR` (:443) plus a 308 redirect listener on `HTTP_ADDR` (:80) that can serve ACME challenges from `ACME_WEBROOT`; otherwise plain HTTP on `ADDR`. Certificates hot-reload on file change (`httpapi/tls.go`).
- **Health:** adapters contribute `health.Check`s (fx group `health`); `/readyz` runs them, `/healthz` is liveness.
- **Redis must stay ElastiCache-compatible:** cluster mode disabled is assumed (Lua scripts touch keys they compute), and no Redis 7.4-only commands (presence uses a hash + sorted set instead of `HEXPIRE`).
- **Persistence:** pgx; migrations run under a Postgres advisory lock (several instances may start at once); transactions via `Transactor.WithinTx` which stores the tx in `ctx` — repositories always use `db.q(ctx)`. Schema = goose migrations embedded from `adapter/postgres/migrations/*.sql` (new file per change, never edit applied ones).
- **Auth:** JWT access token (15 min, in memory on the client, `Authorization: Bearer`) + rotating opaque refresh token in an HttpOnly cookie scoped to `/api/auth`, stored hashed in Redis with family-based reuse detection (Lua in `redisstore/tokens.go`). Logout denylists the access token's `jti`.
- **Search:** OpenSearch only returns matching card IDs; filtering/ordering/cursor pagination (`position:id`) happen in Postgres. The index is rebuilt on start if counts differ.
- **Files:** RustFS via AWS SDK (`s3store.ObjectStore`). Content type is sniffed; only image/video are served inline, everything else as a download. Deleting cards/projects/orgs also deletes their objects.
- **Organizations are tenants** (`docs/omiteam-integration.md`, decision 17): every request works in one organization (`X-Organization`, resolved by `RequireAuth` via `auth.InOrg`; `ErrOrgUnavailable` when the user isn't an active member). `currentUser(c)` carries `OrgID` and the membership's company role there, so use cases filter every query by `viewer.OrgID` — new tables with tenant data need an `organization_id` and repos must take it. Accounts (name, email, password) are shared across organizations. `PlatformAdmin` (env `PLATFORM_ADMINS`) is installation-wide.
- **Company roles** (OmiTeam integration): every membership has a role (`OWNER…CANDIDATE`) + addition roles (`LEAD`) in `domain/company.go`, which is the only copy of the permission matrix (`user.Can(domain.Perm…)`). `/api/auth/me` sends the granted `permissions`; the frontend checks them with `can()` and maps roles to app sections in `lib/companyRoles.ts` (`RequireSection`, `homeRoute`). Project roles (owner/admin/member of a project) are separate. A few sections are gated by a permission instead of the role (`backwoods`, `candidates`, `hrStatistics` in `canAccess`).
- **OmiTeam modules** (use case packages): `people` (team, profiles, documents), `tasks` (task details, planning), `timetracking`, `absence` (vacations, holidays), `reports` (time/billability/bench, task statistics), `hr` (candidates, `hr_events` written also by `user.Dismiss` and `people.Create`), `wiki` (+ project comments; access rules are pure functions in `domain/wiki.go`), `backwoods` (admin panel). Seed data for all of them is in `adapter/postgres/seed.go`.
- **Backups** (`usecase/backwoods`, `adapter/postgres/backwoods.go`): tar.gz of `schema.json` + a CSV per table from one REPEATABLE READ snapshot under an advisory lock, stored under `backups/` in the bucket, never deleted; a goroutine started by the module's fx lifecycle makes a CRON backup when the last is older than `BACKUP_EVERY`. Runtime versions come from the fx group `versions` (`health.ProvideVersion`) that adapters contribute to.
- **Organizations → projects → board.** Projects belong to an org; only its members can join a project. The frontend keeps the chosen organization in localStorage (`lib/api.ts`: `currentOrgId`, `switchOrganization`), shows `OrgSwitcher` in the sidebar, and lists only that organization's projects.

## Real time

One WebSocket endpoint `/api/ws` (`transport/httpapi/realtime*.go` hub — registry, `_conn`, `_fanout`, `_board`, `_voice` — + `usecase/collab`). First message is `{type:"auth", token, projectId}`; `projectId: 0` opens the **user channel** (access changes), otherwise a **board connection**. Fan-out across server instances goes through Redis pub/sub (`board:<id>`, `user:<id>` in `redisstore/realtime.go`).

- Use cases publish `domain.BoardEvent` / `domain.UserEvent` after committing; `redisstore` encodes them as `contract.*`, the hub converts them to DTOs. REST requests carry `X-Realtime-Conn` (the tab's connection id) → events get `origin`, and only the originating tab skips them (not "same user" — a user may have several tabs).
- Cursors are sent as anchors (`{kind: card|column|board, id, fx, fy}`), not pixels.
- Card title/description are a Yjs document per card. The server never parses Yjs: it keeps a numbered update log in Redis (`ydoc:<cardId>`, seeding claimed with SETNX, compaction via Lua) and relays updates. Postgres markdown stays the source of truth and is autosaved by the editing client.
- Removing someone's access publishes `RemovedUserID` / `project.deleted`; the hub then closes their board sockets.
- **Voice rooms** (`usecase/voice`, `frontend/src/lib/voice.ts`, `components/VoiceDock`, `CallBar`, `VoiceRoomBadge`): WebRTC full mesh (≤ `domain.VoiceRoomLimit` people), signaled over the **user channel** (not the board socket) so calls survive navigation; the participant id is that tab's user-channel `connId`. Server state: `voice:<projectId>` (hash + expiry ZSET, same helpers as presence) and `voice:user:<userId>` (one room per user; joining elsewhere sends `voice.left{reason:"elsewhere"}` to the old tab). Point-to-point messages go through `rtc:<connId>` pub/sub. Room changes are pushed as user events (`type:"voice"`) to every project member. The server never parses SDP. TURN credentials come from `GET /api/voice/ice` (coturn `use-auth-secret`, `TURN_SECRET`). E2E testing needs Chrome with `--use-fake-ui-for-media-stream --use-fake-device-for-media-stream`.

## Frontend

Structure and style rules: `docs/architecture.md` → Frontend. In short: by type — `components/ui/` (primitives: Button, Modal, Field, Panel, Menu…), `components/<Name>/`, `layouts/`, `pages/<Name>/`, `hooks/`, `lib/`, `context/`, `types/`, `styles/`. Every component/page is a folder with `Name.tsx` + `Name.module.scss`; one exported component per file; import via `@/`; no barrel files.

- **Styles:** CSS modules + Sass, camelCase classes, `cx()` from `lib/cx.ts`. Tokens in `styles/_tokens.scss` (use `var(--glass)`, `--accent`, `--success`…, never raw colors); mixins from `styles/_mixins.scss` are auto-injected (`@include mobile`, `touch`, `standalone`, `dark`, `glass`, `glass-panel`, `ellipsis`…) — responsive rules sit next to the rule they change. `styles/global.scss` is the only global sheet (tokens, element defaults, third-party hooks). Style a child component by passing `className` and nesting under the parent's class. Links styled as buttons: `buttonClass()` / `iconButtonClass()`. Non-React code finds elements by data attributes (`data-card-id`, `data-column-id`, `data-board-columns`, `data-column-body`, `data-slash-menu`), never by class.
- `lib/api.ts` — fetch wrapper: token refresh (single-flight), translates error codes, adds `X-Realtime-Conn`.
- `lib/realtime.ts` (`BoardSocket`), `hooks/useBoardRealtime.ts` (peers; cursor moves go to a ref, not state), `lib/cardDoc.ts` (Yjs provider over the board socket), `components/LiveCursors` (rAF loop).
- `components/CardModal` (TipTap + Yjs, ~670 kB) is lazy-loaded from `pages/Board`; keep heavy editor/CRDT imports out of the main bundle.
- **i18n:** `src/i18n/en.ts` is the source of truth; `uk.ts`/`de.ts` are typed as `Translation`, so a missing key fails the build. Plural keys must define `_one/_few/_many/_other` in every locale. User settings (language/theme, `auto` allowed) are stored on the account (`context/preferences.ts`) and mirrored to localStorage for first paint.
- **PWA / mobile:** `vite.config.ts` (`VitePWA`: manifest + Workbox precache; `/api` and health checks are excluded from the navigation fallback), `lib/pwa.ts` (SW registration, install prompt), `lib/touch.ts` (HTML5 DnD polyfill for touch + board-aware edge auto-scroll; drop targets must handle `dragenter` too). The Go SPA handler serves `sw.js`/`manifest.webmanifest`/`index.html` with `no-cache` and `/assets/*` as immutable. Breakpoint: ≤860px (`@include mobile`).

## Conventions

- The user writes in Ukrainian; reply in Ukrainian. Everything in the repository except the Ukrainian UI translations (`frontend/src/i18n/uk.ts`) is English: code comments, docs, backend messages, logs, seed data.
- Signed-in users are redirected away from `/`, `/login`, `/register` to `/home` (`PublicOnly` in `auth.tsx`).
