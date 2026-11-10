## 1. Data model

- [ ] 1.1 Add a migration for `loans` with `user_id`, game reference, borrower, contact, lent date, due date, returned date, notes, and notification timestamps
- [ ] 1.2 Add a partial unique index enforcing at most one open loan per user and game
- [ ] 1.3 Add indexes for the open-loan and overdue queries

## 2. Lifecycle

- [ ] 2.1 Implement lend, return and list operations scoped by `user_id`, with `...Tx` variants
- [ ] 2.2 Derive overdue state at read time from the due date rather than storing it
- [ ] 2.3 Close open loans when a sync removes the game from the collection, with an explanatory note
- [ ] 2.4 Cascade loan removal on game deletion

## 3. Web

- [ ] 3.1 Add lend and return actions to the game detail page
- [ ] 3.2 Show loan state on grid cards and the borrower name in list rows
- [ ] 3.3 Build the loans page with open loans oldest-first, overdue marking, and a returned history section
- [ ] 3.4 Ensure the lend form offers recent borrowers as suggestions and is fully keyboard operable

## 4. Reminders

- [ ] 4.1 Add the reminder query selecting overdue loans not notified within the interval
- [ ] 4.2 Send through `internal/notify` from the existing scheduler and stamp the notification time in the same transaction
- [ ] 4.3 Add `REMINDERS_ENABLED` and the reminder interval to configuration, `.env.example` and the operations docs
- [ ] 4.4 Make the reminder pass a no-op when the notifier is nil

## 5. API and verification

- [ ] 5.1 Add loan list, create and return endpoints with the standard error envelope
- [ ] 5.2 Update `docs/api.md` and the OpenAPI document
- [ ] 5.3 Test that a second open loan is refused and that the first is unchanged
- [ ] 5.4 Test that a reminder is sent once and not repeated after a simulated restart
- [ ] 5.5 Test that another user's loan returns not found rather than forbidden
- [ ] 5.6 Test that an ownership change closes the loan and stops reminders
