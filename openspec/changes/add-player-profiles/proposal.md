## Why

Players are free text: `play_players.name` is a string with a `NOCASE` index
and no canonical record. Typing `Tom`, `tom` and `Thomas` creates three people,
so every "who wins most" question is quietly wrong, and there is no way to look
at a person at all. The play log is the app's data engine; its people are
currently unaddressable.

## What Changes

- Introduce a per-user `players` record with a canonical display name and a set
  of aliases, and link `play_players` to it with a nullable `player_id`. The
  original `name` text is retained so nothing is lost and BGG-imported plays
  stay faithful.
- Resolve names on write: an exact alias match links automatically; an
  unmatched name offers "link to existing player" or "create player". Plays can
  still be saved with an unlinked name, so logging never blocks.
- Add a **merge** action that folds duplicate players into one, rewrites their
  play links, and records the merge in the alias set.
- Add autocomplete to every player input on web and mobile, sourced from the
  user's own history and ordered by recency.
- Add a **player profile** page: plays attended, win rate overall and by game,
  head-to-head record against each other player, first plays witness trips,
  preferred weight band inferred from plays, and last played.
- Backfill existing history at migration time by grouping `play_players.name`
  case-insensitively, leaving ambiguous groups for the user to merge.

### Non-goals

- Linking players to app accounts; that is `add-shared-group-shelves`.
- Global, instance-wide player identity or cross-user visibility.
- Importing BGG player identities as a source of truth.
- Ratings of players, handicaps, or matchmaking.

## Capabilities

### New Capabilities

- `player-profiles`: Canonical players, alias resolution, merging, and the
  per-player statistics view.

### Modified Capabilities

- `play-tracking`: Player entry resolves to canonical players and the stored
  player link; play statistics gain a per-player dimension.

## Impact

- New migration: `players` table, `player_aliases` table, `play_players.player_id`
  column and index, plus the backfill.
- `internal/plays`: player resolution on create/update, merge operation, player
  aggregate queries; `...Tx` variants for merge inside one transaction.
- `internal/web/templates`: player profile page, merge screen, autocomplete on
  the play form.
- `internal/api`: player resources and player statistics.
- `mobile/lib/src/screens`: autocomplete in the play form, player profile.
- `docs/api.md`, OpenAPI document.
