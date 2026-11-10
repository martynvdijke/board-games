## Why

The shelf already stores `price_paid`, `acquisition_date`, and the BGG
`wishlist`, `want_to_play`, `preordered` and `for_trade` flags, but they are
invisible: a filter value, two stat counters, and nothing else. The result is a
collection app that cannot answer "what is this hobby costing me?" or "which
games should leave the shelf?" — the two questions collectors ask most.

## What Changes

- Add a **pipeline** view grouping games by acquisition state: wanted, want to
  play, preordered, owned-unplayed, for trade. Reuse the existing
  `user_games` flags; no new columns.
- Add **value per use** to the game detail page and as sortable columns:
  `price per play` and `price per hour`, derived from `price_paid`,
  `plays.quantity` and `plays.duration_minutes`. Games with zero plays show as
  undefined, not zero.
- Add a **trade candidates** list: owned, not played (or rated low), paid above
  a threshold, optionally flagged for trade — ranked with a stated reason.
- Surface `acquisition_date`, `price_paid`, `location` and `condition` on the
  game detail page and in the edit form, with price entered in normal currency
  units and stored as integer minor units.
- Add read-only API access to the pipeline and per-game value fields.
- Extend statistics with collection value totals, median cost per play, and
  unplayed capital.

### Non-goals

- Price history, price watching, or retailer integration.
- Multi-currency conversion; one instance currency is assumed and stored in
  configuration, not per game.
- Automatic valuation from marketplace data.
- Depreciation accounting or tax reporting.

## Capabilities

### New Capabilities

- `collection-value`: Acquisition pipeline states, cost-per-use metrics, trade
  candidate ranking, and the value totals shown in statistics.

### Modified Capabilities

- `game-shelf`: The overview gains a pipeline view and value-aware sorting; the
  detail page and edit form expose acquisition fields.
- `http-api`: New read-only pipeline resource and value fields on game
  resources.

## Impact

- `internal/collection/repo.go`: aggregates for cost per play and pipeline
  counts; new sort keys added to the whitelist.
- `internal/collection`: new service methods and view models.
- `internal/web/templates`: pipeline page, detail-page value block, edit form
  fields, statistics additions.
- `internal/api`: pipeline endpoint and expanded game payload.
- `internal/config`: instance currency code.
- `web/src/style.css`: pipeline and value layout using existing tokens.
- `docs/api.md`, OpenAPI document, `README.md`, `.env.example`.
- No schema migration; no change to BGG sync behaviour.
