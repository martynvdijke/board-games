## Why

Game nights happen in basements, and the Flutter companion currently needs the
network to write a play. When logging fails, the data engine that every
statistic and recommendation depends on quietly starves. Adding a game is also
pure typing, when the box in hand already carries a barcode.

## What Changes

- Add an **offline play queue**: plays composed without connectivity are stored
  locally with a client-generated id and replayed when the API is reachable,
  with exponential backoff and a visible pending count.
- Make play creation idempotent so a retry after a timeout cannot duplicate a
  play, and resolve the case where the same play was later edited on the web.
- Add a **sync status** surface in Settings and an inline indicator in the plays
  list for anything still queued, failed, or awaiting a conflict decision.
- Add **barcode capture**: scan an EAN/UPC code, resolve it through BGG's search,
  and add the game to the collection — with a manual search fallback when the
  code is unknown or the game is already owned.
- Keep credentials in secure storage, keep the queue encrypted at rest, and
  never queue bearer tokens in the payload.

### Non-goals

- Offline editing or deletion of existing plays.
- Full offline collection browsing beyond what is already cached.
- Background sync while the app is terminated.
- Scanning shelf photos to identify multiple games at once.

## Capabilities

### New Capabilities

- `mobile-capture`: Offline play queue, retry and conflict handling, sync
  status, and barcode-driven game capture.

### Modified Capabilities

- `http-api`: Play creation accepts a client idempotency key and reports a
  deterministic conflict for a repeated create.

## Impact

- New migration: a per-user idempotency key column on `plays` with a partial
  unique index, or an `idempotency_keys` table.
- `internal/api`: honour `Idempotency-Key` on `POST /api/v1/plays`, returning
  the existing play on replay.
- `mobile/lib/src/api`: queue persistence, retry, conflict resolution.
- `mobile/lib/src/screens`: sync status in settings, capture entry point.
- `mobile/pubspec.yaml`: local database and barcode scanning dependencies.
- `docs/mobile.md`, `docs/api.md`, OpenAPI document.
