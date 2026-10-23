## Context

`board-games` is a brand-new repository. The family already uses BoardGameGeek for collection and play tracking, but BGG is slow to browse, visually dated, and offers no pleasant way to answer "what should we play tonight?" or "what have we owned for years and never played?" The existing personal projects in `~/projects` establish a house style: Go services, SQLite, server-rendered HTML, Vite/TypeScript page bundles, Tailwind (or Bootstrap), Playwright e2e, Docker, semantic versioning, and Flutter companions.

The deployment target is a single-user instance behind an existing reverse proxy, with TLS and hostname handled by the proxy. The schema and authentication model still support multiple users so family members can be added later without a migration.

BoardGameGeek exposes only a read API (XML API2). There is no supported endpoint for writing plays, ratings, or collection status. The user has chosen local-only writes for v1: BGG is the source for collection metadata and historical plays, while new plays and personal state live in this application.

## Goals / Non-Goals

**Goals:**

- A beautiful, fast shelf overview with instant search, filters, sorting, grid/list density, and dark mode.
- OIDC authentication with database-backed sessions, roles, per-user collection state, and scoped personal API tokens.
- Idempotent, resumable BGG read-sync for collection, game metadata, and plays, with visible progress and friendly failure handling.
- Local play logging with participants, scores, and a timeline; lightweight collection and play statistics.
- A documented `/api/v1` consumed by future clients, plus token-scoped TRMNL JSON for an e-ink display.
- A single container that runs with SQLite on a mounted volume, with health checks, OpenTelemetry, and optional Umami analytics.
- Warm editorial design that feels personal and delightful without sacrificing accessibility or performance.

**Non-Goals:**

- BGG writeback, browser automation, or stored BGG credentials.
- Private BGG collections (v1 requires a public collection; documented in settings).
- Invites, registration, billing, or social features.
- Android companion (milestone 2, separate change after v1).
- Lending/loans, barcode scanning, market-price tracking, notifications, and real-time collaboration.
- A client-side SPA, native mobile web app, or offline PWA.

## Decisions

### D1: Server-rendered Go with Vite/TypeScript islands (not an SPA)

Go `net/http` serves `html/template` pages; Vite builds TypeScript entry points per page into `static/js`; Tailwind compiles the stylesheet. Interactive behaviour (instant search, filters, view toggle, rating widget, play dialog) is hand-written TypeScript with no framework.

**Why:** This matches `meetup`, `synapse`, and `dnd`, keeps one deployable and one auth model, gives fast first paint for image-heavy cover grids, keeps search state in the URL, and leaves SEO and TRMNL rendering trivial. A Vue/React SPA would duplicate auth (cookie + bearer), add a router, and delay the first cover appearing. A separate SvelteKit/Next app would add a second deployment for no benefit.

### D2: Go 1.27 stdlib ServeMux and `modernc.org/sqlite`

Use the standard library router with method-pattern routes (`GET /games/{id}`), `modernc.org/sqlite` (pure Go, as in `meetup` and `dnd`), embedded SQL migrations, and hand-written queries. FTS5 is available in the pure-Go driver and is verified by an integration test in the foundation milestone.

**Why:** No CGO makes the Docker build simple and the binary portable. The stdlib router is sufficient for this route count and keeps middleware explicit. An ORM (`ent`) would add codegen and obscure the FTS queries that are central to the product.

### D3: Shared game catalogue + per-user shelf rows

`games` stores BGG metadata once per `bgg_id`, shared across users. `user_games` stores each user's status flags, rating, notes, acquisition details, and BGG play count. `plays` and `play_players` are per-user. Junction tables hold categories, mechanics, designers, and publishers for exact filtering.

**Why:** BGG metadata is global and expensive to fetch; shelf state is personal. Sharing the catalogue keeps sync cheap and makes future family accounts instant. Storing categories/mechanics as JSON would make filtering and FTS fragile.

### D4: OIDC-only web login with DB-backed sessions and scoped tokens

Authorization Code + PKCE, discovery from `OIDC_ISSUER`, state/nonce validation, ID-token signature/audience checks. Sessions are random 256-bit tokens stored as SHA-256 hashes, delivered in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie with a configurable TTL. CSRF uses a per-session token plus an origin check for unsafe methods. Personal API tokens (`bg_…`) are shown once, stored hashed, scoped `read` / `write` / `trmnl`, and revoked by setting `revoked_at`.

API authentication accepts, in order: `Authorization: Bearer bg_…` personal token, session cookie, or a provider-issued JWT validated against the issuer's JWKS (for clients that support OIDC natively). The first authenticated user becomes `admin`; an optional `ALLOWED_EMAILS`/`ALLOWED_DOMAINS` allowlist limits access.

**Why:** OIDC matches the existing infrastructure and avoids password storage. Opaque tokens are revocable and work for TRMNL, which cannot perform an interactive flow. JWT support covers native mobile clients without forcing them to copy a token.

### D5: Read-only, idempotent, rate-limited BGG sync

BGG base URL is environment configuration, never user input. A process-wide limiter allows ~1 request/second. Requests carry a descriptive `User-Agent` including a contact address. The client treats `202 Accepted` and `429/503` as retryable with jittered backoff, and polls a queued collection until it returns `200`.

Sync phases per user:

1. `collection` — `GET /xmlapi2/collection?username=<u>&stats=1`; parse status flags, personal rating, BGG play count, and basic metadata. Upsert `games` and `user_games`.
2. `things` — batch `GET /xmlapi2/thing?id=a,b,c&stats=1` for games whose `fetched_at` is missing or older than 30 days; parse description, image, players, playtime, age, weight, rank, categories, mechanics, designers, publishers. Batch size 20, one request per second.
3. `plays` — paginate `GET /xmlapi2/plays?username=<u>&page=N` until the last page; upsert by `(user_id, bgg_play_id)`.
4. `finalize` — recompute derived counts, refresh FTS, mark the run complete.

Each phase commits per page/batch, so a failure leaves consistent partial data and the next run resumes. `sync_runs` records phase, progress, counts, timings, and a human-readable error. Manual "Sync now" and a scheduled interval (default 6h, per user, disableable) share the same service and a per-user mutex prevents concurrent runs. A `202` storm or BGG outage surfaces as a retryable status, not a crash.

**Why:** BGG is slow and flaky; a synchronous request-time sync would time out. Per-batch commits make retries safe. Dedicated `bgg_play_id` uniqueness makes re-sync idempotent.

### D6: FTS5 plus exact junction filters

`games_fts` is a standalone FTS5 table (`name, description, designers, publishers, categories, mechanics`, `tokenize='unicode61 remove_diacritics 2'`) whose `rowid` equals `games.id`. It is refreshed inside the same transaction as catalogue writes. A safe query builder escapes operators and appends `*` to the final token for prefix matching. Filters (players, playtime, weight, category, mechanic, status) use indexed joins and `user_games` columns; sorting is whitelisted, never interpolated.

**Why:** FTS5 gives fast, forgiving search over titles and descriptions; exact filters need normalized tables. Whitelisted sort/filter columns prevent SQL injection.

### D7: URL-backed UI state and progressive enhancement

Search, filters, sort, density, and page live in the query string. The page renders a complete server-side result; TypeScript intercepts input, debounces, fetches an HTML fragment (`/partials/games`) or JSON, and swaps the grid, updating `history.replaceState`. Without JavaScript, the form still submits and works. Hover-only actions are duplicated on focus and in the detail view.

**Why:** Shareable URLs, working back/forward, resilient behaviour, and no client-side state library.

### D8: TRMNL as a token-scoped JSON projection

`GET /api/v1/trmnl?token=…` (or `Authorization: Bearer`) returns a compact payload: `stats`, `pick`, `recent`, and `shelf_of_shame`. The pick is deterministic per day and user (seeded hash of date + user) so the device does not flicker between polls; weighting favours short playtime, supported player counts, unplayed status, and high personal rating, with a human-readable `reason`. Cover URLs are absolute and proxied through the app (`/media/bgg/{id}.jpg`) so the device never depends on BGG's CDN. `/trmnl/preview` renders the same data as an 800×480 e-ink-style page for tuning.

**Why:** TRMNL private plugins poll JSON and need stable URLs and absolute images. Deterministic picks make the display feel intentional. A preview removes guesswork.

### D9: Warm editorial design system

**Palette (light):** background `#F7F2E8` parchment, surface `#FFFDF8`, ink `#1F1B16`, muted `#6F6558`, line `#E3D9C8`, primary felt green `#1F5A45`, primary hover `#174534`, brass accent `#C97B3C`, danger `#9B3A32`.
**Palette (dark):** background `#14120F`, surface `#1D1A16`, ink `#F1EADD`, muted `#A79C8C`, line `#332D26`, primary `#7FBFA3`, brass accent `#E0A468`.

**Typography:** self-hosted variable `Newsreader` for display and game titles; `DM Sans` for UI and metadata; tabular numerals for stats. Scale 12/14/16/18/22/28/40/56 with fluid display sizes. **Spacing:** 4px base; 8/12/16/24/32/48/64. **Radii:** 10px cards, 6px inputs, pill chips. **Shadow:** `0 1px 2px rgba(31,27,22,.06), 0 12px 32px -12px rgba(31,27,22,.18)`. **Motion:** 180ms ease-out; 8px page fade; 2px card lift; honor `prefers-reduced-motion`.

Cover art is the hero: 2:3 covers, warm inner border, title/year beneath, meta badges for players/time/weight, brass rating chip, and an "Unplayed" ribbon. The overview masthead greets the user by time of day, states collection facts in plain language, and features one "On the table" pick. Empty states use friendly line-art meeples and specific copy. Dark mode follows `system` by default with a manual override persisted per user.

**Why:** The product is a personal shelf, not an admin dashboard. Warm materials, serif titles, and large covers make it feel curated and gift-worthy; a neutral catalogue would be forgettable. Tokens keep the system consistent and testable.

### D10: Single-user deployment, multi-user schema

The app listens on an internal port (`:6290` by default) and trusts forwarded headers only from configured proxy CIDRs. `BASE_URL` drives OIDC redirects, cookies' secure flag, and absolute TRMNL/media URLs. TLS terminates at the proxy. No user-management UI ships in v1; the settings page exposes only the current user's BGG account, tokens, and theme. The `users.role` column and per-user scoping remain, so family accounts work by allowlisting their IdP emails.

**Why:** The user explicitly deploys one instance for themselves. Keeping per-user keys avoids a painful migration if that changes, while avoiding unnecessary admin surface area.

### D11: OpenTelemetry and Umami

OTLP traces, metrics, and logs are wired through `OTEL_*` env vars (same pattern as `meetup`/`heat`), with spans around HTTP requests, BGG calls, sync phases, and database hot paths. `/metrics` exposes Prometheus format when enabled. Umami is optional: a self-hosted script tag on HTML pages (`UMAMI_URL`, `UMAMI_WEBSITE_ID`) plus a tiny event client for future clients. No personal data or game titles are sent as event properties.

**Why:** Consistent operations with the rest of the portfolio and honest, privacy-friendly analytics.

### D12: Testing pyramid with a fake BGG server

- Go unit tests for XML parsers, FTS query building, token hashing/scopes, pick weighting, and config.
- Handler/API tests against a temporary SQLite database and `httptest` fake BGG endpoints (queued `202`, success, malformed XML, throttle, pagination).
- Sync idempotency tests: run twice, assert row counts and play dedupe.
- Playwright e2e against the built binary with `DEV_FAKE_AUTH` and `BGG_BASE_URL` pointing at the fake server: login, sync, search/filter, shelf of shame, log play, token-protected TRMNL, dark mode.
- Vitest for TS URL-state serialization, debounce, rating widget, and relative dates.
- An axe-core accessibility check on overview, game detail, and settings.

**Why:** External BGG behaviour is the main source of flakiness; a fake server makes sync deterministic. Auth is mocked only in e2e, while unit tests exercise real state/nonce logic.

## Risks / Trade-offs

- **[BGG rate limits or outages make first sync slow]** → one-request-per-second limiter, batched `thing` calls, per-batch commits, resumable runs, visible progress, retry with jitter, and a 30-day metadata refresh window.
- **[BGG changes or blocks the XML API]** → isolate parsing in `internal/bgg`, pin fixtures, keep the base URL configurable, and surface actionable errors. No credentials are stored, so there is no account risk.
- **[BGG has no write API, so users expect pushes]** → settings and docs state read-only clearly; the data model keeps `bgg_play_id` and BGG-compatible fields so an export can be added later without migration.
- **[Private BGG collections cannot sync]** → validate at save time and explain that the collection must be public; do not ask for BGG passwords.
- **[Cover hotlinking breaks or leaks browsing to BGG]** → proxy and cache thumbnails locally, fall back to a generated placeholder with the game's initial and palette colour.
- **[FTS5 unavailability in the pure-Go driver]** → verify in the foundation milestone; fallback is `LIKE`-based search behind the same interface.
- **[Single-user assumption bakes in shortcuts]** → all data access is scoped by `user_id` from day one, even though the UI hides multi-user administration.
- **[Image-heavy overview is slow on mobile]** → responsive `srcset`/sized thumbnails, `loading="lazy"`, `decoding="async"`, fixed aspect ratios to prevent layout shift, and pagination at 48 games.
- **[Reverse-proxy misconfiguration breaks OIDC/TRMNL URLs]** → require `BASE_URL`, document trusted proxy CIDRs, and log the effective redirect URI at startup.
- **[Local-only plays diverge from BGG]** → label local plays clearly and show BGG-imported vs local origin; writeback/export is a documented future change.

## Migration Plan

This is a new deployment with no existing data. On startup the server creates the database file if missing and applies embedded migrations in order, recording versions in `schema_migrations`. All migrations are additive and forward-only for v1.

Deployment steps:

1. Build the image and create the `/data` volume.
2. Configure `BASE_URL`, OIDC issuer/client, `SESSION_SECRET`, and BGG `User-Agent`.
3. Start behind the reverse proxy; confirm `/healthz` and the OIDC callback.
4. Sign in, save the BGG username, run the first sync, and verify the shelf.

Rollback: stop the container, restore the previous image, and restore the SQLite file from backup. Because v1 is the first schema, no down-migrations are provided; backups are taken with `sqlite3 .backup` (documented in the README). Future changes must be additive or include a migration note.

## Open Questions

None blocking v1. Product name defaults to "The Shelf" and is configurable through `APP_NAME`; the Android companion, BGG export, and lending are deferred to later changes.
