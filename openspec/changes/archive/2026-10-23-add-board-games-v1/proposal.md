## Why

The family's board game collection lives on BoardGameGeek, where browsing owned games, remembering what has not been played, and finding something that fits tonight's group is slow and visually dated. There is no single, beautiful place that shows the shelf at a glance, keeps play history locally, and exposes the collection to a companion app and an e-ink display. This change builds that place: a self-hosted, Go-backed shelf with BGG read-sync, OIDC authentication, a documented API, and a TRMNL feed.

## What Changes

- Create a new Go service with an embedded SQLite database, server-rendered pages, and a Vite/TypeScript/Tailwind frontend.
- Add OIDC login with database-backed sessions, role support, and personal API tokens. The app is deployed as a single-user instance behind an existing reverse proxy but keeps per-user data isolation so family accounts can be added without a migration.
- Sync a user's public BoardGameGeek collection, game metadata, and play history into local tables. Sync is read-only and idempotent; BGG writeback is explicitly deferred because BGG has no public write API.
- Provide a cover-forward shelf overview with instant search, filters (players, playtime, weight, category, mechanic, status), sorting, grid/list views, and a playful "shelf of shame" for owned games with zero plays.
- Add local play logging with players and scores, a play timeline, and lightweight collection/play statistics.
- Expose a versioned REST API with OpenAPI documentation and token authentication for the future Android companion.
- Add a token-protected TRMNL JSON feed and HTML preview that picks a game for the evening and summarises the shelf on an 800×480 e-ink screen.
- Add Docker packaging, health endpoints, OpenTelemetry instrumentation, and optional Umami analytics.
- Add a responsive warm editorial design system (parchment/felt-green, serif display type, cover-first cards, dark mode, reduced-motion and keyboard support).

## Capabilities

### New Capabilities

- `oidc-authentication`: OIDC login, session management, roles, per-user isolation, and scoped personal API tokens.
- `game-shelf`: Shared game metadata and per-user collection state, BGG search/filter/sort, game detail, shelf-of-shame, and the web UI design system.
- `bgg-sync`: Read-only BoardGameGeek collection/thing/plays synchronisation with rate limiting, retries, idempotency, and visible sync status.
- `play-tracking`: Local play logging with participants and scores, play timeline, and collection/play statistics.
- `http-api`: Versioned JSON API, bearer authentication, pagination, consistent errors, and OpenAPI documentation.
- `trmnl-feed`: Token-protected e-ink payload and preview for TRMNL devices.
- `operations`: Container packaging, reverse-proxy deployment, configuration, health checks, OpenTelemetry, and Umami analytics.

### Modified Capabilities

<!-- This is a new repository; no existing capabilities change. -->

## Impact

- **New code**: `cmd/server`, `internal/{config,db,auth,bgg,collection,plays,api,trmnl,web,observability}`, `templates/`, `web/` (Vite/TS/CSS), `db/migrations/`, `e2e/`.
- **Data**: new SQLite schema for `users`, `sessions`, `api_tokens`, `user_settings`, `games`, catalogue junction tables, `user_games`, `plays`, `play_players`, `sync_runs`, and a `games_fts` FTS5 index.
- **External systems**: BoardGameGeek XML API2 (read-only, public collections only); configured OIDC provider; optional OTLP collector and Umami instance.
- **Frontend dependencies**: Vite, TypeScript, Tailwind CSS, self-hosted Newsreader and DM Sans fonts. No client-side SPA framework.
- **Backend dependencies**: Go 1.27, `modernc.org/sqlite`, OpenTelemetry SDK/OTLP exporters, `golang.org/x/oauth2` and OIDC verification libraries.
- **Operations**: multi-stage Docker image, Compose example, `/data` SQLite volume, reverse-proxy trust configuration, `BASE_URL`-based absolute URLs.
- **Deferred**: BGG writeback/automation, private BGG collections, the Flutter Android companion, invites/registration, lending, barcode scanning, and market-price tracking.
