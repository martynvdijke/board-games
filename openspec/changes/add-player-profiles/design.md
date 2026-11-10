## Context

`play_players` stores `name TEXT NOT NULL` with an index on
`name COLLATE NOCASE`, plus `score`, `is_winner`, `is_first_play` and
`position`. There is no player entity. Plays are written through
`internal/plays` with an `insertPlayers` helper, read back through `players` and
`playersPage`, and BGG-imported plays are immutable (`origin='bgg'`).

Constraints: user-scoped everything, `db.WithTx` for multi-statement work, no
nested transactions, and BGG plays must remain faithful to what BGG returned.

## Goals / Non-Goals

**Goals:**
- Make a person addressable without breaking any existing play or import.
- Resolve names automatically where it is safe, and ask where it is not.
- Prove that merging cannot orphan or duplicate a play.

**Non-Goals:**
- Player accounts or cross-user identity.
- Rewriting stored `name` text, including on BGG-imported plays.
- Ratings, handicaps, or matchmaking.

## Decisions

**Keep `play_players.name` as the recorded fact and add a nullable
`player_id`.** BGG sends a name string; rewriting it into a canonical value would
falsify an imported record. The canonical player is a *link*, and the display
name lives on `players`. Unlinked rows keep working everywhere because every
existing query reads `name`.

**Alias table, not a name-matching heuristic at runtime.**
`player_aliases(player_id, alias COLLATE NOCASE)` with a unique index on
`(user_id, alias)`. Resolution is a single indexed lookup. Fuzzy matching is
offered in the UI as a suggestion but never applied automatically, because a
silent wrong merge is worse than a duplicate.

**Backfill groups case-insensitively and links only unambiguous groups.** The
migration creates one player per distinct `lower(name)` per user, links all
matching rows, and leaves near-duplicates (`Tom` / `Thomas`) as separate players
for the merge UI. Deterministic and reviewable, unlike attempting string
similarity in SQL.

**Merge runs inside one `db.WithTx`.** Steps: repoint `play_players.player_id`
from source to target, move aliases (ignoring conflicts), delete the source
player, insert an audit row. A unique index on `(play_id, player_id)` cannot be
violated because two different players cannot already occupy the same play.

**Player statistics are aggregates over `play_players` joined to `plays`.**
Win rate, games played, head-to-head and weight preference are all
`GROUP BY` queries scoped by `user_id`; head-to-head is a self-join on `play_id`
restricted to rows where one side is the subject player.

**Autocomplete is a separate lightweight endpoint, not part of the play list.**
Returns at most ten players by recency then frequency, so the play form stays
fast and can be cached client-side per session.

## Risks / Trade-offs

- **Duplicate players persist because merging is manual** → acceptable and
  visible; the profile page offers "possible duplicates" derived from shared
  play dates and similar names.
- **A mis-merge is hard to reverse** → record the audit row and keep the alias
  list from the merged player so the original names remain searchable; document
  that merge is not currently undoable in the UI.
- **Statistics cost on large logs** → aggregates are bounded by the user's own
  plays; add `idx_play_players_player` on `player_id`.
- **BGG sync writing new plays with unknown names** → resolution runs on the same
  code path, so sync auto-links exact aliases and creates players otherwise,
  without ever modifying the BGG name.

## Migration Plan

1. Create `players` and `player_aliases`, add `play_players.player_id` nullable.
2. Backfill per user from `lower(name)`.
3. Deploy code that writes `player_id` on new plays and tolerates NULL on old
   ones.
Rollback: the new tables and column are additive; old code ignores them.

## Open Questions

- Should a play with an unresolved name block saving? Current decision: no.
- Do we want a per-player "colour" for the UI, or reuse the existing avatar
  initials treatment? Defer to implementation with the design system.
