## 1. Data model

- [ ] 1.1 Add a migration for `players` and `player_aliases` with a unique index on `(user_id, alias COLLATE NOCASE)`
- [ ] 1.2 Add nullable `play_players.player_id` with an index, and a unique index on `(play_id, player_id)`
- [ ] 1.3 Backfill one player per distinct lowercased name per user and link matching rows
- [ ] 1.4 Leave near-duplicate names unmerged and add a test asserting the backfill links every row in its group

## 2. Resolution and merge

- [ ] 2.1 Add alias-based resolution used by play create and update, storing the typed name unchanged
- [ ] 2.2 Add a merge operation that repoints links, moves aliases, deletes the source and writes an audit row, inside one `db.WithTx`
- [ ] 2.3 Expose merge through the web handler with CSRF protection
- [ ] 2.4 Add `...Tx` variants used by the merge and by import

## 3. Autocomplete and profile

- [ ] 3.1 Add the player suggestion query (recency then frequency, limit 10) and a small JSON endpoint
- [ ] 3.2 Wire autocomplete into the web play form as a TypeScript island
- [ ] 3.3 Build the player profile page with plays, win rate, per-game win rate, head-to-head, first plays, weight band and last played
- [ ] 3.4 Suppress rate-based sections below three plays
- [ ] 3.5 Suggest likely duplicates from shared play dates and similar names

## 4. API and mobile

- [ ] 4.1 Add player resources and per-player statistics to the API
- [ ] 4.2 Add player autocomplete to the Flutter play form
- [ ] 4.3 Add a player profile screen to the mobile app
- [ ] 4.4 Update `docs/api.md` and the OpenAPI document

## 5. Verification

- [ ] 5.1 Test that resolution links exact aliases case-insensitively and never auto-merges unmatched names
- [ ] 5.2 Test that a merge failure leaves both players and all links unchanged
- [ ] 5.3 Test that a BoardGameGeek play's stored name is byte-identical after resolution
- [ ] 5.4 Test player statistics aggregate spelling variants into one player
- [ ] 5.5 Verify player data of one user is unreachable from another user's requests
