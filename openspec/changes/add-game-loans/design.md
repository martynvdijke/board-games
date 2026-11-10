## Context

`internal/notify` defines a `Notifier` interface with a Telegram implementation,
called fire-and-forget from a background goroutine. Two schedulers exist:
`internal/collection/sync.go` uses a one-minute ticker for sync work, and
`cmd/server/main.go` uses a six-hour ticker. Games are owned rows in
`user_games`; a game can be removed from the collection by a later sync.

Constraints: single SQLite writer, no job queue, no nesting transactions, and
notifications must never fail a request.

## Goals / Non-Goals

**Goals:**
- Answer "who has my game?" in one screen and one column on the shelf.
- Remind without spamming, and without requiring a new notification transport.
- Keep loan state correct when ownership changes underneath it.

**Non-Goals:**
- Borrower accounts or borrower-side acknowledgement.
- Deposits, insurance, or replacement value.
- Any reminder path that runs inside an HTTP request.

## Decisions

**One `loans` row per lending event, with a partial unique index for the open
loan.** `CREATE UNIQUE INDEX ... ON loans(user_id, game_id) WHERE returned_at IS
NULL` gives the database the "at most one open loan" invariant instead of
relying on application checks. History is preserved as closed rows.

**Borrower is a name plus optional contact, not a foreign key to `players`.**
Lending is often to someone who will never appear in a play log. Keeping it text
avoids a dependency on `add-player-profiles` and matches how people describe it.
When player profiles exist, the form offers them as autocomplete only.

**Reminders run in a scheduler pass, guarded by a `notified_at` column.** Each
tick selects overdue open loans where `notified_at IS NULL OR notified_at <
now - reminder_interval`, sends, then stamps. This makes a double-send
impossible without a queue, and survives a restart because the guard is in the
database.

**Notifications are opt-in and no-op when unconfigured.** `RemindersEnabled`
defaults off; the notifier is nil unless Telegram is configured. The same
fire-and-forget contract is reused so a slow Telegram API cannot stall the tick.

**Overdue is a derived state, never stored.** A loan is overdue when
`returned_at IS NULL AND due_at IS NOT NULL AND due_at < today`. Storing it would
require a daily job just to keep a boolean honest.

**An open loan is closed automatically when the game leaves the collection.**
On sync completing, any open loan whose `user_games.owned` became false is
returned with a note. This prevents reminders about a game the user no longer
has, which would be worse than no reminder.

**API mirrors the web rules.** Loan create/return are state-changing and require
CSRF on the browser path, bearer auth on the API path, exactly like plays.

## Risks / Trade-offs

- **Reminder fatigue** → one reminder per loan by default, with a configurable
  interval and a hard opt-in; no "still overdue" daily nagging.
- **Borrower name typos** → free text, but autocomplete and a per-user recent
  borrowers list reduce drift.
- **Scheduler runs in every process** → the app is single-container by design; if
  that ever changes, the `notified_at` guard plus a transaction makes duplicate
  sends unlikely but not impossible. Note this in the operations doc.
- **A loan outliving a game deletion** → `ON DELETE CASCADE` on the game
  reference, with the closed row removed. Losing loan history for a deleted game
  is acceptable; keeping an orphan is not.

## Migration Plan

1. Add the `loans` table and indexes.
2. Deploy code; the scheduler pass is inert until `REMINDERS_ENABLED` is set.
3. Rollback: drop the table; no other feature reads it.

## Open Questions

- Should the shelf card show a borrower name or just an icon? Icon on the grid,
  name in the list view, pending design review.
- Do we want a per-game "lendable" flag for irreplaceable games? Out of scope for
  now.
