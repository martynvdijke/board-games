## Context

`user_games` already carries `wishlist`, `want_to_play`, `preordered`,
`for_trade`, `price_paid` (integer minor units), `acquisition_date`,
`location` and `condition`. They are imported by BGG sync and surfaced only as
filter cases in `internal/collection/repo.go` and two counters in the statistics
query. Plays carry `quantity` and `duration_minutes`, so cost per use is a pure
aggregate over existing rows.

Constraints: SQLite only, no ORM, sort clauses must stay whitelisted, all
queries filter by `user_id`, and the cost metrics must not produce misleading
zeros.

## Goals / Non-Goals

**Goals:**
- Turn existing acquisition columns into two finished views and a set of
  sortable metrics.
- Keep every new query parameterised and user-scoped.
- Make cost metrics honest when a denominator is missing.

**Non-Goals:**
- New acquisition columns or a second source of truth for collection state.
- Currency conversion, price history, or external valuation.
- Any write path that conflicts with BGG sync ownership of shared fields.

## Decisions

**Compute cost per use at query time, not in a stored column.** A stored
`cost_per_play` would have to be invalidated on every play insert, edit, delete
and price change, and would go stale silently during sync. A single aggregate
subquery joins `plays` grouped by `game_id`; the collection page is already
paginated, so the join touches at most one page of game ids.
*Alternative rejected:* a materialised column with triggers — more moving parts,
and the value is cheap to compute.

**Represent an undefined metric as NULL, never 0.** A game with no plays has no
cost per play, and a play with no duration has no cost per hour. Sorting puts
NULLs last consistently. In templates the value renders as an em dash with a
title explaining the missing input.

**Price is entered in currency units and stored as integer minor units.** The
existing column is an integer; the form accepts `54.00` and converts once at the
boundary. The instance currency comes from configuration (`SHOP_CURRENCY`,
default `EUR`) and is never stored per game, so a wrong setting is fixable in one
place.

**Pipeline is a view over flags, not a new state machine.** The five states are
derived: wanted = `wishlist`, want to play = `want_to_play`, preordered =
`preordered`, for trade = `for_trade`, owned-unplayed = `owned AND no plays`.
A game can appear in more than one bucket, which matches how BGG users actually
flag things; the page shows counts per bucket and never implies exclusivity.

**Trade candidates are ranked by an explicit, published rule.** Score combines
unplayed status, absence from the last N plays, paid amount, user rating relative
to the collection median, and the `for_trade` flag. Each candidate shows the
reason it ranked, mirroring the explainability already used by "on the table".

**New sort keys extend the existing whitelist.** No string interpolation of
sort clauses; each key maps to a fixed SQL fragment.

## Risks / Trade-offs

- **Aggregate cost on large shelves** → the per-page join is bounded by page
  size; add an index on `plays(game_id)` if absent and verify with `EXPLAIN
  QUERY PLAN` in a test.
- **Statistics query grows** → keep value totals in a separate query rather than
  widening the existing single-row scan.
- **Mixed interpretations of `price_paid`** → audit seeded and imported data
  before shipping; document that the column is minor units and backfill nothing.
- **Sorting NULLs inconsistently across databases** → SQLite puts NULLs first on
  ASC; use an explicit `IS NULL` ordering term rather than relying on defaults.

## Migration Plan

No schema migration. Deploy is a code change plus configuration. Rollback is
reverting the release; no data is written by this change except user edits to
price and acquisition date, which the current edit path already tolerates.

## Open Questions

- Should the pipeline replace the "wishlist" filter or sit alongside it? Current
  preference: alongside, so existing URLs keep working.
- Is `acquisition_date` ever useful without `price_paid`? If not, the form can
  require neither and simply show whichever exists.
