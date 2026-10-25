# The Shelf

A quiet, warm home for the board games you own, the plays you remember, and the
ones still waiting for their first outing.

The Shelf syncs a public BoardGameGeek collection, keeps a local play log and
shelf notes, and exposes the whole thing through a small REST API and a
token-protected TRMNL display feed. It is built as a single Go binary with
server-rendered HTML, a little TypeScript, and SQLite. No cloud account, no SPA,
no client-side database.

## What it does

- **A shelf worth browsing.** Cover-forward grid, compact list, instant search,
  filters for players, time, weight, category, mechanic, and ownership state,
  plus a deliberately opinionated "on the table" pick.
- **Shelf of shame, gently.** Unplayed games are counted, filterable, and
  surfaced with how long they have been waiting.
- **Plays that stay yours.** Log plays with players, scores, winners, duration,
  location, and notes. Plays imported from BGG are read-only; locally logged
  plays can be edited or deleted.
- **BoardGameGeek in, not out.** BGG's public XML API2 is read-only. The Shelf
  imports your public collection, metadata, ratings, and play history, but it
  never writes back. See [BoardGameGeek limits](#boardgamegeek-limits).
- **One collection, several doors.** Server-rendered web UI, documented REST
  API with personal tokens, and an 800×480 TRMNL feed.
- **Made to live on a shelf.** Single-user friendly, multi-user capable, OIDC
  sign-in, one SQLite file, a non-root container, OpenTelemetry, and optional
  Umami analytics.

## Quick start (native)

Requirements: Go 1.27+, Node 24+, and a BoardGameGeek username.

```sh
cp .env.example .env
# Set a real SESSION_SECRET and your OIDC provider details in .env.
# For a local look around, set DEV_FAKE_AUTH=true and skip OIDC entirely.
set -a; . ./.env; set +a

npm install
npm run build
go run ./cmd/seed            # optional: 12 demo games and some plays
go run ./cmd/server
```

Open <http://localhost:6290>. With `DEV_FAKE_AUTH=true`, visiting
`/auth/login` signs you in as `owner@example.com` without an identity provider.
Never enable fake auth in production.

### Try a sync without touching BoardGameGeek

```sh
go run ./cmd/fakebgg -addr :6291
BGG_BASE_URL=http://127.0.0.1:6291/xmlapi2 go run ./cmd/server
```

The seed command sets the demo user's BGG username to `alexplays`, which the
fake server understands.

## Docker

```sh
cp .env.example .env
# Edit .env. At minimum set SESSION_SECRET, BASE_URL, OIDC_* and BGG_USER_AGENT.
docker compose up -d --build
```

The compose file binds to `127.0.0.1:6290` by default, ready for an existing
reverse proxy. Data lives in the `shelf-data` volume at
`/data/boardgames.db`. Set `BIND_ADDR=0.0.0.0` only if you really want the
container exposed directly.

See [docs/deployment.md](docs/deployment.md) for Caddy and nginx examples,
proxy trust, and OIDC callback configuration.

## Sign-in and users

The first user to sign in becomes an admin. Additional users are created on
first successful OIDC login. There is deliberately no invitation or user
management UI in v1. Restrict access with either:

```env
ALLOWED_EMAILS=you@example.com,partner@example.com
# and/or
ALLOWED_DOMAINS=example.com
```

Sessions are database-backed, hashed at rest, HttpOnly, SameSite=Lax, and
Secure in production. CSRF tokens protect every state-changing browser request.

## Personal API tokens

Create a token in **Settings → API tokens**. Tokens start with `bg_`, are
shown once, stored only as SHA-256 hashes, and carry scopes:

| Scope | Grants |
| --- | --- |
| `read` | `GET` access to your shelf, games, plays, stats, and sync status |
| `write` | Read plus shelf edits, play create/update/delete, and starting syncs |
| `trmnl` | The dedicated TRMNL feed only |

Bearer tokens use `Authorization: Bearer bg_...`. OIDC access tokens are also
accepted when the provider exposes a verifiable JWT. Requests are rate-limited
per user (120/minute) with `Retry-After` on 429.

```sh
curl -H "Authorization: Bearer $TOKEN" https://shelf.example.com/api/v1/games?limit=5
```

Full reference: `/api/docs` in the running app, or the OpenAPI JSON at
`/api/openapi.json`. See also [docs/api.md](docs/api.md).

## TRMNL

The feed is designed for an 800×480 e-ink panel:

```sh
curl -H "Authorization: Bearer $TRMNL_TOKEN" \
  https://shelf.example.com/api/v1/trmnl
```

It returns today's stats, a deterministic daily pick with a human reason, five
recent plays, and the shelf of shame. The token needs the `trmnl` scope. A
pixel-exact preview is at `/trmnl/preview`, and all image URLs are absolute and
proxy-aware.

## Android companion

A Flutter companion app lives in [`mobile/`](mobile/). It signs in with a
personal API token (create one under **Settings → API tokens**) and offers the
shelf, game details, play logging, statistics, shelf edits, sync progress, and
the same warm visual language.

Download `the-shelf-<version>-android.apk` from a GitHub release, or build it:

```sh
cd mobile
flutter pub get
flutter build apk --release
adb install build/app/outputs/flutter-apk/app-release.apk
```

Release builds need Java 17. Without a `mobile/android/key.properties` the APK
is debug-signed, which is fine for sideloading but not for store distribution.
The app supports optional OpenTelemetry (OTLP/HTTP) and Umami export via
compile-time dart-defines. See [docs/mobile.md](docs/mobile.md) for signing,
connection, telemetry, and the privacy-safe analytics event list.

## BoardGameGeek limits

BoardGameGeek's XML API2 has **no official write API**. The Shelf therefore:

- reads `/collection`, `/thing`, and `/plays` from BGG;
- imports ratings and play history;
- stores all local ratings, notes, statuses, and new plays only in its own
  SQLite database;
- does **not** update your BGG collection, log plays on BGG, or write ratings;
- cannot read private BGG collections.

The app identifies itself with `BGG_USER_AGENT`, which must include a contact
address in parentheses, and rate-limits requests to roughly one per second.
Use a descriptive value such as `TheShelf/1.0 (you@example.com)`.

Sync happens on demand and on a per-user schedule (default every six hours, or
six hours after the last successful run). Collection and thing metadata are
refreshed at most every 30 days; play history is idempotent by BGG play ID.

## Configuration

All configuration is environment-based. The complete annotated list lives in
[`.env.example`](.env.example). The important ones:

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_NAME` | `The Shelf` | Product name in the UI |
| `ENV` | `development` | `production` enables secure cookies, JSON logs, HSTS |
| `PORT` | `6290` | HTTP listen port |
| `BASE_URL` | `http://localhost:6290` | Public URL used for OIDC and absolute image links |
| `DATABASE_PATH` | `./data/boardgames.db` | SQLite file; parent directory is created |
| `SESSION_SECRET` | dev-only generated | Required in production, 32+ random bytes |
| `OIDC_ISSUER` | — | Issuer URL; discovery is automatic |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | — | OIDC client credentials |
| `OIDC_REDIRECT_URL` | `BASE_URL/auth/callback` | Override only if it differs |
| `ALLOWED_EMAILS` / `ALLOWED_DOMAINS` | empty | Optional allowlist |
| `BGG_USER_AGENT` | dev placeholder | Must contain a contact in parentheses in production |
| `BGG_SYNC_INTERVAL` | `6h` | Default schedule for new users |
| `TRUSTED_PROXIES` | loopback | CIDRs allowed to set forwarded headers |
| `METRICS_ENABLED` | `true` | Expose Prometheus metrics at `/metrics` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | Optional OTLP trace/metric export |
| `UMAMI_URL` / `UMAMI_WEBSITE_ID` | empty | Optional privacy-friendly analytics |
| `VERSION` | `dev` | Build/release version shown in logs |
| `DEV_FAKE_AUTH` | `false` | Local/e2e only; never in production |

Secrets are redacted from the startup config log.

## Development

```sh
task install          # Go, npm, Playwright, frontend build
task seed             # demo data
task run              # server on :6290
task fakebgg          # fake BGG API on :6291
task test             # Go tests
task test:web         # Vitest
task test:e2e         # Playwright with a fresh seeded DB
task check            # gofmt, vet, Go tests, typecheck, Vitest, e2e, production build
```

The frontend is deliberately small: Vite + TypeScript + Tailwind 4, built into
`internal/web/static` and embedded in the binary. There is no React/Vue runtime.
For asset watching, run `npm run dev` alongside a Go server that has been built
at least once.

### Project shape

```text
cmd/server          HTTP entrypoint and graceful shutdown
cmd/seed            representative local data
cmd/fakebgg         offline BGG XML API for tests and demos
internal/auth       OIDC, sessions, CSRF, personal tokens
internal/bgg        XML API2 client, data models, fake server
internal/collection catalogue, shelf queries, sync orchestration
internal/plays      play repository, validation, statistics
internal/trmnl      feed builder, deterministic pick, image proxy/cache
internal/api        REST API, OpenAPI, docs
internal/web        templates, handlers, embedded Vite assets
internal/db         SQLite open/pragmas and migrations
internal/observability  logs, traces, metrics, request middleware
web/                TypeScript and Tailwind source
e2e/                Playwright and axe tests
openspec/           the change specification and task history
```

Data model in one breath: a shared `games` catalogue (plus category, mechanic,
designer, and publisher junctions and an FTS5 index) joined to per-user
`user_games` shelf state; `plays` and `play_players` record history; `sync_runs`
records progress; `users`, `sessions`, `api_tokens`, and `user_settings` hold
identity and preferences.

## Observability and analytics

OpenTelemetry is wired through the HTTP server, BGG client, sync orchestration,
SQLite transactions, and play creation. Set `OTEL_EXPORTER_OTLP_ENDPOINT` to
export traces and metrics; Prometheus metrics are also available at `/metrics`.
Incoming W3C `traceparent` headers are honoured, `X-Trace-ID` is returned to the
caller, and HTTP logs carry request, trace, and span IDs. The OpenTelemetry
service name is configurable with `OTEL_SERVICE_NAME` and defaults to
`boardgames`.

Umami is entirely optional. Set both `UMAMI_URL` and `UMAMI_WEBSITE_ID` and the
script is loaded with a CSP that allows both its script and its `/api/send`
beacon. The app emits a deliberately small, allowlisted set of events:

| Event | Properties |
| --- | --- |
| `theme_changed` | `theme` |
| `shelf_search` | `has_query` (boolean only) |
| `play_submitted` | `mode` (`create`/`edit`) |
| `sync_started` | none |

Game names, search text, notes, and player names are never sent. Unknown events
or properties are dropped in the client helper before they reach Umami.

## CI, releases, and dependency updates

- **CI** (`.github/workflows/ci.yaml`) runs on every pull request and non-main
  push: frontend build, TypeScript typecheck, Vitest, `gofmt`, `go vet`, Go
  tests with coverage, and the full Playwright + axe suite.
- **Release** (`.github/workflows/release.yaml`) runs CI on `main`, then uses
  Semantic Release to tag versions, update `CHANGELOG.md`, `package.json`, and
  `package-lock.json`, and publish `martynvandijke/board-games` and
  `ghcr.io/martynvdijke/board-games` with both `latest` and the release version.
  The version is baked into the image through the `VERSION` build argument and
  exposed to the app as an environment variable.
- **Renovate** (`.github/workflows/renovate.yml` + `renovate.json`) checks Go,
  npm, Docker, and GitHub Actions updates three times a day and automerges
  non-0.x updates.
- **Workflow lint** runs actionlint, zizmor, and pinned-action checks; stale
  branches are archived and cleaned up weekly.

Repository secrets are managed from the homelab central secrets file. On the
host that owns that file:

```sh
~/homelab/scripts/add-repo-secrets.sh ~/projects/board-games --dry-run
~/homelab/scripts/add-repo-secrets.sh ~/projects/board-games
```

The script sets the global OTLP, Docker Hub, Gotify, Renovate, and TRMNL
secrets for this repository. `GITHUB_TOKEN` is provided automatically by
Actions. No application credentials are stored in the repository.

## Operations

- `GET /healthz` is liveness, `GET /readyz` checks the database.
- `/metrics` exposes Prometheus metrics when enabled.
- SIGINT/SIGTERM drains HTTP, stops the scheduler, and closes SQLite cleanly.
- SQLite uses WAL, foreign keys, a five-second busy timeout, and normal sync.
- Backups and restore: [docs/backup.md](docs/backup.md).

## Licence

Private project. BoardGameGeek data remains subject to BGG's terms. Game covers
are proxied and cached locally to avoid hot-linking.
