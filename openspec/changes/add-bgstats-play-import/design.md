## Context

Plays are created through `internal/plays` with `origin IN ('local','bgg')`, a
partial unique index on `(user_id, bgg_play_id)` for idempotent sync, and
immutability enforced for `origin='bgg'`. Games are matched by BGG id. Players
are free text today, and `add-player-profiles` introduces canonical players.

Constraints: SQLite, no background job queue, no nested transactions, and a
wrong import must not corrupt a user's existing log.

## Goals / Non-Goals

**Goals:**
- Move years of history in one upload, safely and reversibly per play.
- Never duplicate a play, even if the same export is uploaded twice.
- Report precisely what will and will not be imported before writing anything.

**Non-Goals:**
- Importing collections, ratings, or wishlists.
- A live BG Stats integration.
- Bulk undo of an accepted import.

## Decisions

**Two-phase import: dry run then commit, with the same code path.** The dry run
produces a report and a signed, short-lived import session id holding the parsed
rows server-side; commit references that session. This avoids re-uploading and
re-parsing, and makes the commit deterministic.

**Idempotency key derived from source data, not a random id.** For each row,
compute a stable key from
`sha256(date|game_bgg_id|sorted players|scores|quantity)` truncated to 32 hex
chars, stored as `import_key` with a partial unique index on `(user_id,
import_key)`. Re-uploading the same export collides and skips. *Alternative
rejected:* a per-file hash, which misses a re-export where one row was edited.

**`origin` gains `'import'`.** A check-constraint migration recreates the table
(the SQLite pattern for altering checks), copies rows, and swaps. Imported plays
are immutable in the same way as BGG plays, so the existing guard generalises to
`origin <> 'local'`.

**Game matching is BGG id first, exact normalised name second, and never fuzzy.**
Unmatched rows appear in the report as "needs a game" with a picker; a fuzzy
match that silently attaches a play to the wrong game is worse than an extra
click.

**Commit is one transaction per batch of at most 200 plays.** Each play, its
players, and its key insert together. A failure rolls back the batch, not the
whole import, and the report records which batch failed so the user can retry
after fixing one row.

**Player resolution reuses the profile resolver but never blocks.** Names link to
existing aliases where possible; otherwise a player is created during commit,
because an import with unlinked players would fragment statistics immediately —
the exact problem the import is meant to fix.

**Dates are parsed as local date plus optional time, then stored as RFC3339
UTC.** An ambiguous date format is detected per column during mapping
(`DD/MM/YYYY` versus `MM/DD/YYYY`) by checking for values greater than 12 in the
first component; if the sample is ambiguous, the user chooses before commit.

## Risks / Trade-offs

- **Wrong date interpretation misplaces history** → mandatory mapping preview
  showing the first five parsed dates in full, plus a count of rows the chosen
  format cannot parse.
- **A large export times out** → server-side import session and batched commits;
  the report is streamed as sections, not one giant JSON blob.
- **Duplicate detection is too aggressive** → key includes scores and quantity,
  so two genuinely identical plays on the same day with the same scores would be
  skipped. Surface that count explicitly in the report and allow "import anyway"
  for the batch with keys left null.
- **Constraint migration on a large `plays` table** → run inside one transaction,
  verify row count before and after, and keep the migration idempotent.

## Migration Plan

1. Recreate `plays` with the widened `origin` check and a nullable `import_key`
   plus partial unique index.
2. Deploy parser, dry run and commit behind a feature route.
3. Rollback: the added column is nullable and ignored by old code; the check
   constraint only widens.

## Open Questions

- Should the import session live in the database or in server memory? Memory is
  simpler but lost on restart; leaning toward a short-lived table row.
- Do we need an "undo this import" affordance for the first week after upload?
  Tempting, but it conflicts with the immutability rule; prefer a clear report.
