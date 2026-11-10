## 1. Data model and service

- [ ] 1.1 Add a migration for `table_sessions` with game, owner, attendees, mode, state, timestamps and a link to the created play
- [ ] 1.2 Implement start, snapshot, finish and discard operations scoped by `user_id`
- [ ] 1.3 Convert a finished session into a play inside one `db.WithTx` using the existing play creation path
- [ ] 1.4 Expire drafts untouched for twenty-four hours on read

## 2. Session page

- [ ] 2.1 Add the session template with large targets, high contrast and a dim-room variant derived from existing tokens
- [ ] 2.2 Build the score entry island with real buttons, `inputmode` fallback and polite score announcements
- [ ] 2.3 Build the session clock with pause and resume, persisting elapsed time locally
- [ ] 2.4 Persist a debounced snapshot to the server and merge on reload by monotonic counter
- [ ] 2.5 Add the finish summary showing scores, duration and the created play link

## 3. Scoring modes

- [ ] 3.1 Implement competitive scoring with highest or lowest wins
- [ ] 3.2 Implement cooperative mode recording a table win or loss with no individual winner
- [ ] 3.3 Implement team mode with per-team scores and team members marked as winners
- [ ] 3.4 Default the mode from known cooperative data and allow per-session override
- [ ] 3.5 Add optional round markers that append rather than replace totals and summarise into play notes

## 4. Entry points and fallbacks

- [ ] 4.1 Start a session from the game detail page and from a planner result
- [ ] 4.2 Provide a clear path to the standard play form when JavaScript is unavailable
- [ ] 4.3 Guard against double-submit on finish

## 5. Verification

- [ ] 5.1 Test that no play exists before finishing and exactly one exists after
- [ ] 5.2 Test reload, offline edit and double-tap finish merge behaviour
- [ ] 5.3 Test that a stale draft is abandoned and not listed as active
- [ ] 5.4 Test each scoring mode's winner calculation
- [ ] 5.5 Run axe checks at phone-landscape and tablet-portrait sizes, and verify full keyboard operation
