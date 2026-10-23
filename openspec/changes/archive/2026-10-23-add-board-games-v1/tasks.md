## 1. Foundation and tooling

- [x] 1.1 Create `go.mod` (`module boardgames`, Go 1.27), add `modernc.org/sqlite`, OIDC/OAuth, OpenTelemetry, and UUID dependencies; add `cmd/server/main.go` skeleton
- [x] 1.2 Implement `internal/config` with env parsing, validation, redacted effective-config logging, and a committed `.env.example`
- [x] 1.3 Set up `slog` structured logging, request IDs, and a panic-recovery middleware
- [x] 1.4 Implement the embedded SQL migration runner with `schema_migrations` and transactional startup
- [x] 1.5 Build the base router with middleware chain, `GET /healthz`, `GET /readyz`, and graceful shutdown
- [x] 1.6 Create `package.json`, `vite.config.ts`, `tsconfig.json`, and Tailwind CSS entry point
- [x] 1.7 Create the base `html/template` layout, navigation, flash partial, and asset-manifest helper
- [x] 1.8 Implement design tokens (light/dark) and self-host Newsreader + DM Sans fonts
- [x] 1.9 Add `Taskfile.yml`, multi-stage `Dockerfile`, `.dockerignore`, and `docker-compose.yml`
- [x] 1.10 Verify the foundation builds, runs, and serves the health endpoints

## 2. Database schema

- [x] 2.1 Add migration for `users`, `sessions`, `api_tokens`, and `user_settings`
- [x] 2.2 Add migration for `games`, catalogue junction tables, and the `games_fts` FTS5 index
- [x] 2.3 Add migration for `user_games`, `plays`, `play_players`, and `sync_runs`
- [x] 2.4 Add indexes, foreign keys, check constraints, and partial unique index for BGG play dedupe
- [x] 2.5 Implement database open/pragmas (WAL, busy timeout, foreign keys) and a transaction helper
- [x] 2.6 Add migration and FTS5 integration tests against a temporary database

## 3. Authentication and users

- [x] 3.1 Implement OIDC discovery, PKCE, state/nonce storage, and ID-token verification
- [x] 3.2 Implement `/login`, `/auth/callback`, and `/logout` handlers with return-to handling
- [x] 3.3 Implement the session store (hashed tokens, TTL, cleanup) and authentication middleware
- [x] 3.4 Implement user upsert, first-user admin assignment, roles, and email/domain allowlist
- [x] 3.5 Implement CSRF tokens, origin checks, and secure cookie configuration
- [x] 3.6 Implement personal API token creation, listing, hashing, scopes, last-used tracking, and revocation
- [x] 3.7 Implement bearer authentication for personal tokens and provider JWTs with scope enforcement
- [x] 3.8 Build the login page, auth error states, settings token UI, and theme preference persistence
- [x] 3.9 Add auth unit/handler tests for state mismatch, expired sessions, allowlist, scopes, and cross-user isolation

## 4. BoardGameGeek sync

- [x] 4.1 Define BGG XML models and parsers for collection items
- [x] 4.2 Define BGG XML models and parsers for thing details (description, players, playtime, weight, rank, categories, mechanics, designers, publishers)
- [x] 4.3 Define BGG XML models and parsers for paginated plays and players
- [x] 4.4 Implement the BGG HTTP client with base URL, User-Agent, rate limiter, 202 polling, retry/backoff, and timeouts
- [x] 4.5 Implement collection sync: upsert shared games and per-user shelf state, preserve local edits, and unset removed ownership
- [x] 4.6 Implement metadata enrichment in batches with 30-day refresh and junction/FTS updates
- [x] 4.7 Implement plays sync with `(user_id, bgg_play_id)` idempotency and per-page commits
- [x] 4.8 Implement `sync_runs` recording, phase progress, per-user locking, and resumable failure handling
- [x] 4.9 Implement the scheduled sync worker with per-user interval and enable/disable support
- [x] 4.10 Implement settings handlers for saving/validating usernames, starting sync, and reading status
- [x] 4.11 Build a fake BGG server with collection/thing/plays fixtures; cover 202 queues and 429 throttling with `httptest`
- [x] 4.12 Add tests for parsing, rate limiting, idempotent re-sync, partial failure, and local-edit preservation

## 5. Shelf overview and search

- [x] 5.1 Implement collection queries: search, player count, playtime, weight, category, mechanic, status, whitelisted sort, pagination
- [x] 5.2 Implement FTS query building and result ranking with prefix matching and safe escaping
- [x] 5.3 Implement shelf statistics (owned, unplayed, plays this year, wishlist, longest unplayed)
- [x] 5.4 Build the overview page with masthead greeting, stat strip, "On the table" pick, and empty state
- [x] 5.5 Build game card, grid, compact list, and pagination partials
- [x] 5.6 Implement `/partials/games` fragment endpoint and URL-state serialization/deserialization
- [x] 5.7 Implement instant search, filter chips, sort, density toggle, and debounced result swapping in TypeScript
- [x] 5.8 Build the game detail page with metadata, cover, BGG link, collection state, and recent plays
- [x] 5.9 Implement collection state editing (status flags, rating, notes, acquisition fields)
- [x] 5.10 Implement the shelf-of-shame count, filter, and longest-waiting highlight
- [x] 5.11 Build the statistics page with monthly plays, top games, player leaderboard, and weight breakdown
- [x] 5.12 Implement dark mode, system theme detection, and the theme toggle
- [x] 5.13 Add handler tests for filters, sorting, 404s, collection edits, and user isolation
- [x] 5.14 Add Vitest tests for URL state, debounce, rating widget, and relative dates

## 6. Play tracking

- [x] 6.1 Implement the play repository: create with players, update, delete, list by game/year, and monthly aggregates
- [x] 6.2 Implement play validation (game ownership, date rules, duration, quantity, duplicate players)
- [x] 6.3 Build the log-play form/dialog and web handlers with CSRF protection
- [x] 6.4 Build the play timeline grouped by month with players, scores, winners, and origin labels
- [x] 6.5 Connect play counts and statistics to the overview and stats page
- [x] 6.6 Add play handler/repository tests including cross-user access and edit/delete counts

## 7. REST API and documentation

- [x] 7.1 Build the `/api/v1` router with bearer/session authentication middleware and scope enforcement
- [x] 7.2 Implement `GET /me`, `GET /games`, and `GET /games/{id}` with pagination and filters
- [x] 7.3 Implement `PATCH /games/{id}` with partial shelf-update validation
- [x] 7.4 Implement `GET/POST /plays` and `PATCH/DELETE /plays/{id}`
- [x] 7.5 Implement `GET/POST /sync` and `GET /stats`
- [x] 7.6 Implement the shared error envelope, pagination metadata, and request validation helpers
- [x] 7.7 Implement API rate limiting with `429` and `Retry-After`
- [x] 7.8 Write the OpenAPI 3 document and serve JSON plus a browsable docs page
- [x] 7.9 Add API tests for auth scopes, validation, pagination, error envelopes, and cross-user isolation

## 8. TRMNL and media

- [x] 8.1 Implement the BGG image proxy with disk cache, content-type validation, size limits, and deterministic placeholder
- [x] 8.2 Implement the deterministic recommendation algorithm and reason generation
- [x] 8.3 Implement the TRMNL payload builder (stats, pick, recent, shelf of shame)
- [x] 8.4 Implement the token-protected TRMNL endpoint with JSON content type and cache headers
- [x] 8.5 Build the 800×480 TRMNL preview page
- [x] 8.6 Add tests for token scopes, stable picks across days, empty shelf, and image fallback

## 9. Operations and observability

- [x] 9.1 Implement OpenTelemetry tracing, metrics, and log correlation behind `OTEL_*` configuration
- [x] 9.2 Add HTTP, sync, BGG, and database instrumentation plus `/metrics` when enabled
- [x] 9.3 Implement optional Umami script injection and privacy-friendly event helper
- [x] 9.4 Add security headers (CSP, frame options, referrer policy) compatible with self-hosted assets
- [x] 9.5 Verify graceful shutdown, SQLite checkpointing, and startup failure paths
- [x] 9.6 Build the Docker image and verify it runs against a mounted data volume
- [x] 9.7 Document backup/restore, reverse-proxy trust, and required environment variables

## 10. End-to-end, polish, and release

- [x] 10.1 Set up Playwright configuration, fake-auth mode, and a seeded test database
- [x] 10.2 Add e2e coverage for login, overview, search, filters, sorting, and shelf of shame
- [x] 10.3 Add e2e coverage for BGG sync progress and failure states against the fake server
- [x] 10.4 Add e2e coverage for logging, editing, and deleting a play
- [x] 10.5 Add e2e coverage for API tokens and the TRMNL preview/feed
- [x] 10.6 Run axe-core accessibility checks on overview, game detail, and settings
- [x] 10.7 Polish responsive layouts, focus states, empty states, loading skeletons, and reduced motion
- [x] 10.8 Add a seed/demo command with representative games and plays for local review
- [x] 10.9 Write `README.md` and `AGENTS.md` (setup, config, architecture, BGG limitations, tasks)
- [x] 10.10 Run the full gate: `gofmt`, `go vet`, Go tests, typecheck, Vitest, Playwright, and production build
