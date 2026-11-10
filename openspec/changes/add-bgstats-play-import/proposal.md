## Why

BG Stats is the app most people use to log plays, and it holds years of history.
Until that history can move, switching costs the user their past — which is the
single strongest reason someone stays on another app. An importer converts a
competitor into a source of data gravity for us instead.

## What Changes

- Add a **play import** flow that accepts a BG Stats CSV export, detects and maps
  columns, matches games by BGG id first and name second, and previews the result
  before anything is written.
- Show a dry-run report: plays to create, plays skipped as duplicates, rows that
  failed to match a game, rows with unreadable dates, and the players that would
  be created or linked.
- Write imported plays with `origin='import'`, treated like BGG-imported plays:
  immutable in the UI, editable only by re-import, and never silently
  overwritten by a later BGG sync.
- Make the import idempotent with a per-user import key derived from the source
  row, so the same export can be uploaded twice without duplicating plays.
- Feed matched player names through the canonical player resolution added by
  `add-player-profiles`, creating players only after the dry run is accepted.

### Non-goals

- Importing collections, ratings, or wishlists from BG Stats or any other app.
- Live API integration with BG Stats.
- Undoing an import as a bulk operation; individual plays are deleted normally.
- Supporting arbitrary third-party CSV schemas beyond documented column mapping.

## Capabilities

### New Capabilities

- `play-import`: CSV intake, column mapping, dry-run reporting, idempotent
  creation of imported plays, and player resolution during import.

### Modified Capabilities

- `play-tracking`: The play origin set gains an imported source and the
  immutability rule extends to it.

## Impact

- New migration: extend the `origin` check constraint and add an import key
  column with a partial unique index per user.
- `internal/plays`: import parsing, matching, dry-run and commit paths, with
  `...Tx` variants so a batch commits atomically.
- `internal/web/templates`: upload, mapping and preview screens.
- `internal/api` and `docs/api.md` if the import is exposed beyond the browser.
- Test fixtures: representative BG Stats exports including edge cases.
- `README.md`.
