## Context

The Flutter companion uses Riverpod 3, stores credentials in secure storage, and
talks to the versioned JSON API. Screens: onboarding, shelf, game detail, plays,
play form, stats, settings. The API currently creates plays non-idempotently, so a
retry after a timeout can duplicate.

Constraints: the API is bearer-authenticated for mobile, CSRF does not apply, and
every resource is `user_id` scoped.

## Goals / Non-Goals

**Goals:**
- Log a play with no connectivity and have it arrive exactly once.
- Make pending and failed writes visible instead of silent.
- Add a game by pointing the camera at the box.

**Non-Goals:**
- Offline editing or deleting existing plays.
- Background sync with the app terminated.
- Scanning several games from one photo.

## Decisions

**Client-generated play identity is the idempotency mechanism.** The mobile app
mints a UUIDv4 per queued play and sends it as `Idempotency-Key`. The server
stores it in `plays.idempotency_key` with a partial unique index on `(user_id,
idempotency_key)`. A replay returns `200` with the existing play instead of
creating a second one. *Alternative rejected:* a separate keys table, which adds a
join to the hottest write path for no benefit.

**The queue is a local SQLite table via `drift`, not a file of JSON.** It needs
per-row state, retry counts, and query for "pending count"; a document store would
reimplement that badly. Each row stores the request body, the key, created-at,
attempt count, next-attempt-at, and last error.

**Retry is exponential with jitter and a hard cap.** Delays 2s, 8s, 30s, 2m, 10m,
then stop and mark `needs_attention`. No infinite retry loop draining a battery.

**Conflict rule: the server wins, and the user is told.** If a queued create
returns a play that was later edited on the web, the app keeps the server version
and records that the local payload diverged, surfacing it once as an informational
row rather than silently overwriting. Because the queued operation is always a
*create*, there is no field-level merge to perform.

**Sync triggers are explicit: app foreground, pull-to-refresh, and after a
successful login.** No background isolate, because that would require platform
scheduling and a second credential path for marginal gain.

**Barcode capture resolves the code before it writes anything.** Scan EAN/UPC, ask
BGG search by code, show the matched game with cover and player count, then add to
the collection via the existing add path. If the code is unknown or maps to an
already-owned game, fall back to a text search prefilled with nothing rather than
inventing a game record.

**The queue is encrypted at rest with the same mechanism used for credentials.**
Bodies contain no bearer tokens, only play content and a game id, but they can
contain other people's names, so treat them as personal data.

## Risks / Trade-offs

- **Idempotency column on the hot write path** → nullable, partial index, and the
  API only consults it when the header is present; web traffic is unaffected.
- **Clock skew in queued `played_at`** → store the device timestamp but let the
  user correct it in the row before retry; do not silently replace with server
  time.
- **Barcode database coverage** → many board games lack a registered barcode;
  the manual fallback must be first-class, not an error page.
- **Camera permission denial** → the capture entry point must remain usable via
  search, and never block the screen on a permission prompt at launch.

## Migration Plan

1. Add `idempotency_key` and its index; deploy the API change first, which is
   backward compatible.
2. Ship the mobile release that sends the header and queues offline plays.
3. Rollback of the app is safe; the server column stays inert.

## Open Questions

- Should the queue survive logout? Safer to discard it, since the account could
  change; leaning toward discard with a warning if items are pending.
- Is a widget or share-sheet target worth it after this? Possible follow-up, not
  part of this change.
