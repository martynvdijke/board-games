## Why

Board games leave the house for months and nobody remembers who has what. BGG
has no concept of a loan, so this is the one tracking problem a collection app
can own outright — and it is the only feature here that makes the app useful on
the many days between game nights.

## What Changes

- Add a `loans` record per game: borrower name, optional contact, lent date,
  optional due date, returned date, and notes. At most one open loan per game.
- Add **Lend** and **Mark returned** actions on the game detail page and in the
  shelf list, with the current loan state shown on the card.
- Add a **Loans** page listing open loans oldest-first, with overdue loans
  called out and returned history available.
- Send reminders through the existing `internal/notify` notifier on the existing
  scheduler tick: an overdue nudge to the lender, and a single reminder after a
  configurable idle period for loans with no due date.
- Make reminders opt-in per instance, silent when no notifier is configured, and
  never sent for games the user no longer owns.
- Expose loans read/write through the API with the same scoping rules as plays.

### Non-goals

- Borrower accounts, borrower-side confirmation, or a public loan link.
- Barcode/QR check-out flows.
- Deposits, insurance, or replacement value.
- Reminder channels beyond what `internal/notify` already supports.

## Capabilities

### New Capabilities

- `game-loans`: Loan lifecycle, overdue detection, reminder dispatch, and the
  loans view.

### Modified Capabilities

- `game-shelf`: Game detail and list cards show loan state and lend/return
  actions.
- `http-api`: Loan resources under the versioned API.

## Impact

- New migration: `loans` table with a partial unique index on open loans per
  game, and a `user_id` scope.
- `internal/collection` or a new `internal/loans` package for the lifecycle and
  overdue query; `...Tx` variants used by the scheduler.
- `internal/notify`: reuse the interface; no new transport required.
- `cmd/server/main.go` or the existing sync scheduler: a reminder pass guarded
  by a "last notified" column so a tick cannot double-send.
- `internal/web/templates`: loans page and detail-page actions.
- `internal/api`: loan endpoints.
- `docs/api.md`, OpenAPI document, `README.md`, `.env.example`.
