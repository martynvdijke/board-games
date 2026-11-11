## 1. Data model

- [x] 1.1 Add a migration for `loans` with `user_id`, game reference, borrower, contact, lent date, due date, returned date, notes, and notification timestamps
- [x] 1.2 Add a partial unique index enforcing at most one open loan per user and game
- [x] 1.3 Add indexes for the open-loan and overdue queries

## 2. Lifecycle

- [x] 2.1 Implement lend, return and list operations scoped by `user_id`, with `...Tx` variants
- [x] 2.2 Derive overdue state at read time from the due date rather than storing it
- [x] 2.3 Close open loans when a sync removes the game from the collection, with an explanatory note
- [x] 2.4 Cascade loan removal on game deletion

## 3. Web

- [x] 3.1 Add lend and return actions to the game detail page
- [x] 3.2 Show loan state on grid cards and the borrower name in list rows
- [x] 3.3 Build the loans page with open loans oldest-first, overdue marking, and a returned history section
- [x] 3.4 Ensure the lend form offers recent borrowers as suggestions and is fully keyboard operable

## 4. Reminders

- [x] 4.1 Add the reminder query selecting overdue loans not notified within the interval
- [x] 4.2 Send through `internal/notify` from the existing scheduler and stamp the notification time in the same transaction
- [x] 4.3 Add `REMINDERS_ENABLED` and the reminder interval to configuration, `.env.example` and the operations docs
- [x] 4.4 Make the reminder pass a no-op when the notifier is nil

## 5. API and verification

- [x] 5.1 Add loan list, create and return endpoints with the standard error envelope
- [x] 5.2 Update `docs/api.md` and the OpenAPI document
- [x] 5.3 Test that a second open loan is refused and that the first is unchanged
- [x] 5.4 Test that a reminder is sent once and not repeated after a simulated restart
- [x] 5.5 Test that another user's loan returns not found rather than forbidden
- [x] 5.6 Test that an ownership change closes the loan and stops reminders
