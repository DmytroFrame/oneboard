# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

OneBoard: a kanban app. Go backend (`backend/`) + React/TypeScript SPA (`frontend/`), backed by PostgreSQL, Redis, OpenSearch and RustFS (S3) from `compose.yaml`. `README.md` documents features, configuration, the full REST API and the real-time protocol. Repository: https://github.com/DmytroFrame/oneboard (Go module `github.com/DmytroFrame/oneboard/backend`). The product name is spelled **OneBoard**; technical identifiers (DB, bucket, index, Docker, storage keys) use `oneboard`.

## Commands

```bash
make infra-up        # docker compose up -d (Postgres, Redis, OpenSearch, RustFS, Redpanda); needs OrbStack/Docker running
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

- `internal/domain` — entities, `Error{Kind, Code, Msg}`, and **`catalog.go`: every business error with a stable `code`**. The HTTP layer returns `{"error": <English msg>, "code": <code>}`; the frontend translates `errors.<code>`. Adding an error means adding the code to the catalog *and* to all three frontend locales.
- `internal/usecase/<module>` — business logic. Each package declares the interfaces (ports) it consumes; shared ports (`Transactor`, `Memberships`, `BoardEvents`, `UserEvents`, `RequireMember/RequireOwner`, `WithOrigin`) are in `usecase/usecase.go`. Permission checks live here (outsiders get NotFound, not Forbidden).
- `internal/adapter/*` — each package exports an `fx.Module` that provides concrete types **and binds them to every port they satisfy** with one-line providers like `func(r *CardRepo) board.Cards { return r }` (see `adapter/postgres/db.go`). When a use case gains a new port, the binding must be added in the adapter module, otherwise fx fails at startup with "missing type".
- `internal/transport/httpapi` — chi router. Each handler implements `Routes.Mount(public, private)` and is registered with `asRoutes(...)` in `server.go`. `Presenter` builds DTOs and signs URLs (avatars, attachments) because `<img>`/`<video>` can't send the bearer token.
- `internal/app/app.go` — composition root listing all modules; `bootstrap` seeds demo data and syncs the search index before the server starts.

Key cross-cutting mechanics:
- **TLS:** `TLS_CERT_FILE` + `TLS_KEY_FILE` (both must exist) switch `StartServer` to HTTPS on `HTTPS_ADDR` (:443) plus a 308 redirect listener on `HTTP_ADDR` (:80) that can serve ACME challenges from `ACME_WEBROOT`; otherwise plain HTTP on `ADDR`. Certificates hot-reload on file change (`httpapi/tls.go`).
- **Health:** adapters contribute `health.Check`s (fx group `health`); `/readyz` runs them, `/healthz` is liveness.
- **Redis must stay ElastiCache-compatible:** cluster mode disabled is assumed (Lua scripts touch keys they compute), and no Redis 7.4-only commands (presence uses a hash + sorted set instead of `HEXPIRE`).
- **Persistence:** pgx; migrations run under a Postgres advisory lock (several instances may start at once); transactions via `Transactor.WithinTx` which stores the tx in `ctx` — repositories always use `db.q(ctx)`. Schema = goose migrations embedded from `adapter/postgres/migrations/*.sql` (new file per change, never edit applied ones).
- **Auth:** JWT access token (15 min, in memory on the client, `Authorization: Bearer`) + rotating opaque refresh token in an HttpOnly cookie scoped to `/api/auth`, stored hashed in Redis with family-based reuse detection (Lua in `redisstore/tokens.go`). Logout denylists the access token's `jti`.
- **Search:** OpenSearch only returns matching card IDs; filtering/ordering/cursor pagination (`position:id`) happen in Postgres. The index is rebuilt on start if counts differ.
- **Files:** RustFS via AWS SDK (`s3store.ObjectStore`). Content type is sniffed; only image/video are served inline, everything else as a download. Deleting cards/projects/orgs also deletes their objects.
- **Organizations → projects → board.** Projects belong to an org; only org members can join a project. Org role matrix (owner/admin/member) is in `domain/organizations.go` and mirrored in `frontend/src/orgs.ts`.

## Real time

One WebSocket endpoint `/api/ws` (`transport/httpapi/realtime.go` hub + `usecase/collab`). First message is `{type:"auth", token, projectId}`; `projectId: 0` opens the **user channel** (access changes), otherwise a **board connection**. Fan-out across server instances goes through Redis pub/sub (`board:<id>`, `user:<id>` in `redisstore/realtime.go`).

- Use cases publish `domain.BoardEvent` / `domain.UserEvent` after committing; the hub converts them to DTOs. REST requests carry `X-Realtime-Conn` (the tab's connection id) → events get `origin`, and only the originating tab skips them (not "same user" — a user may have several tabs).
- Cursors are sent as anchors (`{kind: card|column|board, id, fx, fy}`), not pixels.
- Card title/description are a Yjs document per card. The server never parses Yjs: it keeps a numbered update log in Redis (`ydoc:<cardId>`, seeding claimed with SETNX, compaction via Lua) and relays updates. Postgres markdown stays the source of truth and is autosaved by the editing client.
- Removing someone's access publishes `RemovedUserID` / `project.deleted`; the hub then closes their board sockets.

## Frontend

- `api.ts` — fetch wrapper: token refresh (single-flight), translates error codes, adds `X-Realtime-Conn`.
- `realtime.ts` (`BoardSocket`), `useBoardRealtime.ts` (peers; cursor moves go to a ref, not state), `cardDoc.ts` (Yjs provider over the board socket), `components/LiveCursors.tsx` (rAF loop).
- `CardModal` (TipTap + Yjs, ~670 kB) is lazy-loaded from `Board.tsx`; keep heavy editor/CRDT imports out of the main bundle.
- **i18n:** `src/i18n/en.ts` is the source of truth; `uk.ts`/`de.ts` are typed as `Translation`, so a missing key fails the build. Plural keys must define `_one/_few/_many/_other` in every locale. User settings (language/theme, `auto` allowed) are stored on the account and mirrored to localStorage for first paint.
- Styling is a single `styles.css` with "liquid glass" design tokens on `:root` and dark-theme overrides; use the tokens (`--glass*`, `--panel-shadow`, `--accent`, …) rather than raw colors.

## Conventions

- The user writes in Ukrainian; reply in Ukrainian. Everything in the repository except the Ukrainian UI translations (`frontend/src/i18n/uk.ts`) is English: code comments, docs, backend messages, logs, seed data.
- Signed-in users are redirected away from `/`, `/login`, `/register` to `/home` (`PublicOnly` in `auth.tsx`).
