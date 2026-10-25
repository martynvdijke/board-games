# AGENTS.md

Guidance for coding agents working on The Shelf.

## What this is

A Go 1.27 server-rendered web app for a BoardGameGeek collection, local play
log, REST API, and TRMNL feed. SQLite is the only datastore. The frontend is
Vite 8 + TypeScript 7 + Tailwind 4 with no client framework, bundled into
`internal/web/static` and embedded in the Go binary.

Read `openspec/changes/add-board-games-v1/` before changing product behaviour.
It contains the proposal, design decisions, capability specs, and implementation
tasks. Keep the task checkboxes current when working through that change.

## Commands

```sh
task install       # go mod download, npm install, Playwright browser, frontend
task run           # go run ./cmd/server
task seed          # demo database at ./data/boardgames.db
task fakebgg       # fake BGG API at :6291
task test          # go test ./...
task test:web      # vitest
task test:e2e      # frontend build + playwright (fresh seeded DB)
task mobile:check  # Flutter analyze + tests
task mobile:build  # release APK (Java 17)
task check         # full gate
```

`go build ./...` fails on a fresh checkout until the frontend has been built,
because `internal/web/static` is embedded. Run `npm run build` first. The built
assets are intentionally git-ignored; `.gitkeep` keeps the embed directory
present.

## Architecture rules

- Keep HTTP handlers thin. Business rules belong in `internal/collection`,
  `internal/plays`, or `internal/auth`.
- Do not introduce a client-side framework or an SPA router. Pages are Go
  templates; only search filtering and small interactions are TypeScript
  islands.
- Every state-changing browser request needs the CSRF middleware. Bearer-token
  API requests bypass CSRF and are authenticated separately.
- User-scoped queries must always filter by `user_id`. Cross-user reads should
  behave as 404, not 403, where the resource is private.
- BGG is **read-only**. Never add writeback, credentials, or scraping. Plays
  imported from BGG (`origin='bgg'`) must stay immutable; local plays may be
  edited and deleted.
- Keep all SQL parameterised. Sort clauses are whitelisted in
  `internal/collection/repo.go`.
- Use `db.Now()` (RFC3339 UTC) for timestamps and `db.WithTx` for transactions.
- Do not nest transactions. Repositories expose `...Tx` variants for sync work
  (for example `plays.Repo.UpsertImportedTx`).
- New assets must be imported from `web/src/main.ts` (or another Vite entry) so
  they receive content hashes and appear in the manifest.
- Preserve the warm editorial design system in `web/src/style.css`; use the
  existing tokens and components rather than adding a second visual language.
- Keep WCAG AA contrast, keyboard access, reduced-motion support, and visible
  focus states. Run axe tests after UI changes.

## Data gotchas

- Seeded `games.id` values are odd (1, 3, 5, …) because SQLite
  `AUTOINCREMENT` advances the sequence on upsert conflicts. Never hard-code a
  numeric game ID; discover links or use BGG IDs.
- BGG collection metadata is authoritative for shared game fields. Local
  shelf fields (notes, price, location, condition, favourite) must survive
  re-syncs; tests enforce this.
- `bgg_play_id` is unique per user only when non-null; keep the partial unique
  index intact for idempotent sync.
- The image proxy serves `/media/bgg/{bggID}.jpg`, never remote URLs directly.

## Configuration and security

- Production requires `SESSION_SECRET`, OIDC settings (unless fake auth), and a
  BGG User-Agent containing a contact in parentheses.
- Trust forwarded headers only from `TRUSTED_PROXIES`. Set `BASE_URL` to the
  public URL; it drives OIDC redirects and absolute TRMNL image URLs.
- Never enable `DEV_FAKE_AUTH` outside local development or tests.
- Redact secrets in logs. `Config.LogValue` already does this.
- Do not log emails, tokens, CSRF values, or Authorization headers.

## Tests

- Go: table-driven tests, `httptest` for BGG and API, in-memory SQLite via
  `db.OpenMemory`.
- Frontend: Vitest for pure helpers in `web/src/*.test.ts`.
- Mobile: Flutter analyze and widget/unit tests under `mobile/test`; the
  companion uses Riverpod 3 and stores credentials in secure storage.
- E2E: Playwright with fake auth and a fresh seeded DB. The suite runs serially
  because the sync test intentionally replaces the collection; keep destructive
  tests last (`z-sync.spec.ts`).
- `npm run build` before e2e or Go builds that include the web package.

## Commit style

Conventional Commits, small and focused. Update `README.md`, `.env.example`,
OpenAPI, and OpenSpec artifacts when behaviour or configuration changes.
