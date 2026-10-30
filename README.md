# OneBoard

A real-time kanban board for teams: organizations, projects, live cursors and collaborative editing.
Go (Clean Architecture) + React/TypeScript, backed by PostgreSQL, Redis, OpenSearch and S3.

Repository: <https://github.com/DmytroFrame/oneboard>

```
backend/   Go: chi (HTTP), uber/fx (DI), PostgreSQL (pgx + goose), JWT + Redis (sessions, pub/sub),
           OpenSearch (card search), S3 / RustFS (avatars, attachments), WebSocket (real time)
frontend/  Vite + React + TypeScript + react-router + i18next + TipTap/Yjs: landing "/", sign-in/up,
           home "/home", organizations "/orgs/:id", boards "/projects/:id", settings "/settings"
docs/      deployment notes (AWS)
```

## Getting started

Requirements: Go 1.27+, Node 24+, Docker (e.g. OrbStack).

```bash
make infra-up        # PostgreSQL, Redis, OpenSearch, RustFS, Redpanda (compose.yaml)
make dev-backend     # API on :8080 (in one terminal)
make dev-frontend    # http://localhost:5173, proxies /api and the /api/ws WebSocket to :8080
```

Or build and serve everything from one binary: `make run` → <http://localhost:8080>.

Other targets: `make test` (go vet, go test, frontend type-check/build), `make docker-build`
(production image), `make infra-down`, `make infra-logs`.

The backend reads the repo-root `.env` (shared with docker compose); real environment variables win.
Local credentials and connection strings are in [credentials.md](credentials.md).

### Demo data

An empty database is seeded on start (development only). Sign in as `demo@example.com` / `demo1234`:

- owner of **OneBoard Inc.** (project KAN, 48 cards in the first column to show infinite scroll);
- admin of **Mobile Studio** (project MOB, owned by Olena).

`olena@`, `andrii@`, `maria@` and `taras@example.com` use the same password. Taras is in no organization,
which is handy for trying invitations; `new.hire@example.com` has a pending invitation that is accepted
when someone signs up with that email.

## Features

- **Organizations.** Anyone can create one; a person can belong to several. Roles:

  | | owner | admin | member |
  |---|---|---|---|
  | invite people | ✓ (any role) | ✓ (admin/member) | |
  | remove people | ✓ | members only | |
  | change roles | ✓ | | |
  | create and rename projects | ✓ | ✓ | |
  | delete the organization | ✓ | | |

  Invitations are by email: registered people join immediately, others when they sign up. The last owner
  can't be demoted or removed. People who leave or are removed lose access to all of the organization's
  projects; projects they owned pass to an owner.
- **Projects** belong to an organization and each has a board and a task key (`KAN-1`, `MOB-3`). The
  project owner adds members, but only from the organization. Only project members can be assignees.
- **Boards:** custom columns (add, rename, reorder, delete moving cards elsewhere), drag and drop,
  infinite scroll (pages of 20, cursor pagination), server-side search (OpenSearch: key, title,
  description, word prefixes) and assignee filters.
- **Home `/home`:** stats (organizations, projects, assigned to me, urgent, created by me), "My tasks",
  organizations and projects. Signed-in users are redirected here from `/`, `/login` and `/register`.
- **Real-time collaboration** (see [Real time](#real-time)):
  - avatars of people currently on the board are highlighted, the rest dimmed;
  - Figma-style live cursors with names;
  - when someone opens a task, their cursor flies into the card and becomes their avatar;
  - task title and description are edited together, with other people's colored carets;
  - others see a "ghost" of a card while it is being dragged;
  - every change to cards, columns and members shows up for everyone immediately;
  - people see right away when they are added to or removed from an organization or project.
- **Voice rooms** (WebRTC), one per board, like a voice channel in Discord:
  - the microphone button sits in the board's bottom-right corner, with the avatars of the people already
    in the room next to it, so everyone on the board sees who is talking before joining;
  - inside: who is speaking (rings around avatars), who is muted, mute, microphone choice, leave;
  - you stay in the call while browsing other pages; a bar at the top controls it from anywhere;
  - the sidebar and Home show a 🎧 badge with the number of people next to projects with an active room;
  - joining from another tab or board moves the call there; losing access to the project ends it;
  - joins and leaves play a short chime and show a notice.

  Audio goes peer to peer (a full mesh, up to 8 people per room), so the server carries no audio, only
  signaling (see [Real time](#real-time)). A TURN server (coturn in `compose.yaml`) relays audio for
  people whose networks don't allow a direct connection. Browsers only allow the microphone over HTTPS
  (or on `localhost`).
- **Notion-like task description** (TipTap): `/` slash menu (headings, lists, to-dos, quote, code,
  divider), markdown shortcuts, a floating formatting toolbar. Stored as markdown; existing tasks autosave.
- **Attachments** (up to 50 MB each, 50 per task): the card shows a paperclip with the count and a cover
  (thumbnail of the first image or a frame of the first video). The task dialog has previews, a full-screen
  viewer and drag-and-drop upload. The type is sniffed from the content; image thumbnails (480 px JPEG)
  are generated by the server; videos support Range requests. Anything that isn't an image or video is
  only served as a download (`Content-Disposition: attachment`, `nosniff`, `sandbox`), so uploaded
  HTML/SVG can never run on our origin. File URLs are HMAC-signed and valid for 1–2 days.
- **Avatars:** cropped to 256×256 in the browser, type-checked on the server (PNG/JPEG/WebP/GIF, up to
  5 MB), served from a signed, cache-forever URL.
- **Settings `/settings`:** language (automatic or English/Ukrainian/German) and theme (automatic, light,
  dark), saved to the account so they follow the user across devices.
- **i18n:** `frontend/src/i18n/en.ts` is the source of truth; TypeScript requires `uk.ts` and `de.ts` to
  have exactly the same keys. The backend, logs and seed data are in English.
- **Liquid Glass design** (translucent layers, `backdrop-filter`) in light and dark themes.
- **Mobile and PWA.** Installable as an app (manifest, icons, a service worker via `vite-plugin-pwa`);
  Settings shows an "Install" button, or instructions on iOS. On phones the sidebar becomes a drawer, the
  board shows one column per screen with swipe snapping, dialogs go full screen, and cards are dragged with
  a long press (the `mobile-drag-drop` polyfill; holding near an edge scrolls to the next column). The
  app shell works offline: it opens with the last signed-in user, shows an "offline" bar and reloads data
  when the connection is back. API calls are never cached. Icons are generated from
  `frontend/public/logo.svg` with `npm run icons`.

## Backend architecture

Dependencies only point inwards: `transport → usecase → domain`; adapters implement the use cases' ports.

```
backend/
  cmd/server/main.go       entry point: app.New().Run()
  internal/
    domain/                entities, business errors and the error-code catalog; no dependencies
    usecase/               business logic; each package declares the ports (interfaces) it needs
      auth, user, organization, project, board, attachment, dashboard, collab (real time), voice
    adapter/               port implementations
      postgres/            repositories, transactions, goose migrations (migrations/*.sql), demo data
      redisstore/          refresh tokens, access-token denylist, pub/sub, presence, voice rooms, Yjs logs
      searchindex/         card index in OpenSearch (edge n-grams for prefix search)
      s3store/             object storage (RustFS locally, S3 on AWS) via AWS SDK v2
      jwttoken/            issuing and verifying access tokens (HS256)
      imagethumb/          image thumbnails
      bcrypthash/          password hashing
    transport/httpapi/     chi router, middleware, DTOs/presenter, handlers per module, WebSocket hub
    health/                readiness checks contributed by adapters
    config/                settings from the environment and ../.env
    app/                   composition root: wires all fx modules
```

Every package exports an `fx.Module`. Adapters bind themselves to the ports they satisfy, e.g.
`func(r *CardRepo) board.Cards { return r }`; HTTP handlers join the `routes` group and the server mounts
them all. Replacing an implementation (say, search) means wiring a different module in `internal/app`.

PostgreSQL is the source of truth. OpenSearch only returns the IDs of matching cards; sorting, filtering
and cursor pagination happen in Postgres. On start the index is compared with the database and rebuilt
if the document count differs. Migrations run on start under a Postgres advisory lock.

API errors look like `{"error": "<English message>", "code": "<code>"}`; the frontend shows the
translation of `errors.<code>`. Codes are listed in `backend/internal/domain/catalog.go`.

## Authentication (JWT)

- `login`, `register` and `refresh` return `{user, accessToken, expiresAt}` and set the `refresh_token`
  cookie (`HttpOnly`, `SameSite=Strict`, `Path=/api/auth`, 30 days).
- **Access token:** HS256 JWT valid for 15 minutes (`iss`, `aud`, `sub`, `exp`, `jti`). The frontend
  keeps it in memory only and sends `Authorization: Bearer …`. The API ignores cookies, so CSRF is not
  possible against it.
- **Refresh token:** a random string; Redis stores only its SHA-256. Every `/auth/refresh` rotates it.
  Presenting an already used token revokes the whole session ("family"), except within 30 seconds of a
  rotation, so two tabs refreshing at once don't sign each other out.
- **Logout** revokes the refresh token and puts the access token's `jti` on a Redis denylist until it
  expires, so it stops working immediately.
- The frontend refreshes on its own: shortly before expiry, or once after a 401; concurrent requests
  share one refresh. After a page reload the session is restored via `/auth/refresh`.
- Errors: `token_expired`, `token_invalid`, `session_invalid` (bad refresh token), `not_authenticated`.

## Real time

One WebSocket endpoint, `GET /api/ws`. The first message is `{type: "auth", token, projectId}`, because
browsers can't set an `Authorization` header on WebSockets; foreign origins are rejected at the handshake.

- **Fan-out** goes through Redis pub/sub (`board:<projectId>`, `user:<userId>`, `rtc:<connId>`), so any
  number of backend instances can serve the same board.
- **Events.** After a change through the REST API, the use case publishes a domain event
  (`card.created/updated/moved/deleted`, `columns.changed`, `members.changed`, `project.deleted`) and the
  hub sends it as a DTO. A move carries the card's new neighbours, so clients can place it correctly even
  in a partially loaded column.
- **Origin.** REST requests carry `X-Realtime-Conn` (the tab's connection id); events come back with
  `origin`, and only the tab that made the change skips them — other tabs of the same user apply them.
- **Presence:** a hash plus a sorted set of expiry times per board, refreshed every 20 s; peers of a
  crashed instance are swept out. No Redis 7.4-only commands, so it runs on ElastiCache.
- **Cursors** are sent relative to the card or column under the pointer (`{kind, id, fx, fy}`), not in
  pixels, so they land in the same place on any screen size and scroll position. Clients send at most one
  position per 40 ms (the last one always goes out); the server rate-limits as well.
- **User channel.** Every signed-in tab also opens a connection with `projectId: 0`, subscribed to
  `user:<id>`. It tells the user immediately when they are added to or removed from an organization or
  project, their role changes, or something is renamed or deleted: the sidebar updates, a notice appears,
  pages they can no longer see redirect home, and the server closes their board sockets (code 1008,
  "access revoked") and refuses to reconnect them.
- **Voice rooms** are signaled over the user channel, not the board connection, which is why a call
  survives leaving the board. A participant is that tab's connection (`voice:<projectId>`, a hash plus an
  expiry sorted set like board presence); `voice:user:<userId>` holds the user's current room, so joining
  elsewhere vacates the old one. Messages:

  | Direction | Message | |
  |---|---|---|
  | client → server | `voice.join {projectId, muted}` | checks project membership and the 8-person limit |
  | | `voice.leave`, `voice.mute {muted}` | |
  | | `voice.signal {to, signal}` | SDP offer/answer or ICE candidate for another participant |
  | server → client | `voice.joined {projectId, participants}` | the newcomer dials everyone listed |
  | | `voice.error {projectId, code}` | e.g. `voice_room_full` |
  | | `voice.left {projectId, reason}` | `elsewhere` (joined from another tab/board) or `revoked` |
  | | `voice.signal {from, userId, signal}` | relayed as is; the server never parses SDP |
  | | `user {event: {type: "voice", projectId, participants}}` | sent to every member of the project |

  Signals to one connection go through `rtc:<connId>` pub/sub channels, so the two sides may be on
  different instances. Clients use WebRTC "perfect negotiation" (the lower connection id is the polite
  side), restart ICE when a connection fails, and rejoin after a reconnect.
- **Collaborative editing** with Yjs (CRDT). The server never parses Yjs; it keeps a numbered update log
  in `ydoc:<cardId>`:
  - the first client to open a task gets to seed the document from the Postgres markdown (`SETNX`);
  - the others receive the log and live updates;
  - clients periodically compact the log into a snapshot (atomically, in Lua);
  - the log expires an hour after the last change;
  - Postgres markdown stays the source of truth and is autosaved by whoever edits.

## Configuration

The backend reads `../.env` (or the file in `ENV_FILE`); environment variables take precedence.
`APP_ENV` picks the defaults: `development` (local docker compose) or `production` (AWS — see
[docs/aws.md](docs/aws.md) and [`.env.aws.example`](.env.aws.example)).

| Variable | Development default | Production default | |
|---|---|---|---|
| `APP_ENV` | `development` | | `development` or `production` |
| `ADDR`, `STATIC_DIR` | `:8080`, `../frontend/dist` | `:8080`, `/app/web` (image) | listen address, built SPA |
| `SEED` | `1` | `0` | demo data for an empty database |
| `SECURE_COOKIES` | `0` | `1` | `Secure` refresh cookie (HTTPS) |
| `LOG_FORMAT` | `text` | `json` | |
| `JWT_SECRET` | — (required, ≥ 32 chars) | | signs JWTs and file URLs; changing it ends all sessions |
| `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL` | `15m`, `720h` | | |
| `DATABASE_URL` | built from `POSTGRES_*` | | or `POSTGRES_HOST/USER/PASSWORD/DB` |
| `POSTGRES_SSLMODE`, `POSTGRES_SSLROOTCERT` | `disable` | `verify-full`, RDS CA bundle | |
| `REDIS_ADDR`, `REDIS_USERNAME`, `REDIS_PASSWORD`, `REDIS_DB` | `localhost:6379`, —, from `.env`, `0` | | |
| `REDIS_TLS` | `0` | `1` | ElastiCache in-transit encryption |
| `AWS_REGION` | `us-east-1` | | for S3 and OpenSearch signing |
| `OPENSEARCH_URL` | `https://localhost:9200` | | |
| `OPENSEARCH_AUTH` | `basic` | `aws` without a password | `basic`, `aws` (SigV4) or `none` |
| `OPENSEARCH_USER`, `OPENSEARCH_PASSWORD` | `admin`, `OPENSEARCH_ADMIN_PASSWORD` | | for `basic` |
| `OPENSEARCH_SERVICE` | `es` | | `aoss` for OpenSearch Serverless |
| `OPENSEARCH_INSECURE` | `1` | `0` (forced) | skip TLS verification (local self-signed cert) |
| `OPENSEARCH_INDEX`, `OPENSEARCH_REPLICAS` | `oneboard-cards`, `0` | `oneboard-cards`, `1` | |
| `S3_ENDPOINT` | `http://localhost:9000` | — (AWS S3) | |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `RUSTFS_*` | — (IAM role) | empty = default AWS credential chain |
| `S3_BUCKET`, `S3_REGION` | `oneboard`, `AWS_REGION` | | |
| `S3_FORCE_PATH_STYLE`, `S3_CREATE_BUCKET` | `1`, `1` | `0`, `0` | |
| `STUN_URLS`, `TURN_URLS` | from `.env` (local coturn) | — | comma-separated; without them only direct connections work |
| `TURN_SECRET`, `TURN_CREDENTIAL_TTL` | from `.env`, `12h` | | coturn `static-auth-secret`; per-user credentials are derived from it |

Health endpoints: `GET /healthz` (liveness) and `GET /readyz` (readiness: Postgres and Redis).

### HTTPS with your own certificate (e.g. Let's Encrypt)

| Variable | Default | |
|---|---|---|
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | — | certificate chain and private key (PEM) |
| `HTTPS_ADDR` | `:443` | HTTPS listener, used when both files exist |
| `HTTP_ADDR` | `:80` | answers every request with a `308` redirect to HTTPS |
| `ACME_WEBROOT` | — | serves `/.well-known/acme-challenge/` from this directory instead of redirecting |

When both files are found, the server listens on `HTTPS_ADDR` (TLS 1.2+, HTTP/2) and `HTTP_ADDR` only
redirects to HTTPS, keeping the path and query; `Secure` cookies are switched on. If the variables are set
but a file is missing, it logs why and falls back to plain HTTP on `ADDR`. Without them nothing changes.
The certificate is re-read when the files change (checked once a minute), so renewals need no restart.

```bash
TLS_CERT_FILE=/etc/letsencrypt/live/oneboard.duckdns.org/fullchain.pem
TLS_KEY_FILE=/etc/letsencrypt/live/oneboard.duckdns.org/privkey.pem
ACME_WEBROOT=/var/www/certbot     # optional, for renewals while the server holds port 80
```

Renew with certbot's webroot mode, so certbot doesn't need port 80 itself:
`certbot certonly --webroot -w /var/www/certbot -d oneboard.duckdns.org` (once), then `certbot renew` (cron/timer).

Things to keep in mind:
- Ports below 1024 need privileges on a bare Linux host: run as root or
  `sudo setcap 'cap_net_bind_service=+ep' backend/bin/server`. Docker allows non-root processes to bind them.
- Let's Encrypt keys in `/etc/letsencrypt/archive/` are readable by root only: run the server as root,
  or grant its user read access (e.g. a group with `chmod g+r` on the key and `g+x` on the directories).
- In Docker: `-p 80:80 -p 443:443 -v /etc/letsencrypt:/etc/letsencrypt:ro` (mount the whole directory,
  the `live/` files are symlinks into `archive/`).
- On AWS behind an ALB, terminate TLS on the load balancer instead and leave these variables unset.

## Infrastructure (Docker)

`compose.yaml` runs PostgreSQL, Redis, OpenSearch, RustFS, coturn (TURN/STUN for voice) and Redpanda on
`127.0.0.1` only. Credentials live in `.env`; addresses and connection strings in
[credentials.md](credentials.md). The app uses everything except Redpanda.

## API

| Method | Path | |
|---|---|---|
| POST | `/api/auth/register` | `{name, email, password}` |
| POST | `/api/auth/login` | `{email, password}` |
| POST | `/api/auth/refresh` | refresh cookie → new token pair (rotation) |
| POST | `/api/auth/logout` | revokes the refresh token and, if sent, the access token |
| GET | `/api/auth/me` | current user with `settings: {language, theme}` |
| GET | `/api/dashboard` | stats and the tasks assigned to me |
| GET | `/api/voice/rooms` | `{rooms: {<projectId>: [{connId, userId, name, muted}]}}`, non-empty rooms of my projects |
| GET | `/api/voice/ice` | `{iceServers}` for `RTCPeerConnection`, with TURN credentials valid for `TURN_CREDENTIAL_TTL` |
| GET, POST | `/api/organizations` | my organizations; `{name}` creates one (the creator is owner) |
| GET, PUT, DELETE | `/api/organizations/{id}` | rename: owner/admin; delete: owner |
| GET | `/api/organizations/{id}/members` | members with roles |
| PUT | `/api/organizations/{id}/members/{userId}` | `{role}` — owner only |
| DELETE | `/api/organizations/{id}/members/{userId}` | remove a member, or leave (own id) |
| GET, POST | `/api/organizations/{id}/invitations` | pending invitations; `{email, role}` → `{status: added\|invited}` |
| DELETE | `/api/organizations/{id}/invitations/{invitationId}` | revoke an invitation |
| PUT | `/api/users/me/avatar` | `multipart/form-data`, field `avatar` — PNG/JPEG/WebP/GIF up to 5 MB |
| DELETE | `/api/users/me/avatar` | remove my avatar |
| PUT | `/api/users/me/settings` | `{language: auto\|en\|uk\|de, theme: auto\|light\|dark}` |
| GET | `/api/users/{id}/avatar?v=&sig=` | avatar image; no token, authorised by the URL signature |
| GET | `/api/projects` | the current user's projects |
| POST | `/api/projects` | `{organizationId, name, key}` — org owner/admin; the creator becomes owner |
| PUT, DELETE | `/api/projects/{id}` | `{name}` / delete — project owner only |
| GET, POST | `/api/projects/{id}/members` | project members; `{userId}` — owner only, org members only |
| DELETE | `/api/projects/{id}/members/{userId}` | owner removes anyone, a member removes themselves |
| GET, POST | `/api/projects/{id}/columns` | columns; `{name}` — owner only |
| PUT | `/api/columns/{id}` | `{name}` — owner only |
| POST | `/api/columns/{id}/move` | `{index}` — owner only |
| DELETE | `/api/columns/{id}?moveTo={columnId}` | owner only; `moveTo` is required if the column has cards |
| GET | `/api/projects/{id}/cards?column=&cursor=&limit=&q=&assignees=1,2&unassigned=1` | a page of a column → `{cards, nextCursor, total}` |
| POST | `/api/projects/{id}/cards` | `{title, description, columnId, priority, assigneeId}` |
| GET, PUT | `/api/cards/{id}` | one card; `{title, description, priority, assigneeId}` (description is markdown) |
| POST | `/api/cards/{id}/move` | `{columnId, beforeId?}` or `{columnId, afterId?}`; neither = end of column |
| DELETE | `/api/cards/{id}` | card author or project owner |
| GET, POST | `/api/cards/{id}/attachments` | task files; `multipart/form-data`, field `file` |
| DELETE | `/api/attachments/{id}` | uploader or project owner |
| GET | `/api/attachments/{id}/file\|thumb?exp=&sig=` | file or thumbnail via a signed URL (`&download=1` forces a download) |
| GET (WebSocket) | `/api/ws` | real time, see above |
| GET | `/healthz`, `/readyz` | liveness / readiness |
