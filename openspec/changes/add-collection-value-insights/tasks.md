## 1. Queries and metrics

- [x] 1.1 Add a per-game play and duration aggregate to `internal/collection/repo.go`, scoped by `user_id`, returning play count and summed duration
- [x] 1.2 Compute cost per play and cost per hour in the collection service, returning NULL when a denominator is missing
- [x] 1.3 Add pipeline count and list queries for wishlist, want to play, preordered, for trade and owned-unplayed
- [x] 1.4 Add `cost_per_play` and `cost_per_hour` to the whitelisted sort keys with explicit NULL-last ordering
- [x] 1.5 Implement the trade-candidate ranking as a pure function with a reason string per candidate

## 2. Web

- [x] 2.1 Add the pipeline route and template with per-state headings, counts and game lists
- [x] 2.2 Add the value block to the game detail page, rendering em dashes for undefined metrics
- [x] 2.3 Extend the collection list with value columns, hidden in compact view
- [x] 2.4 Add price and acquisition date to the edit form with unit/minor-unit conversion at the boundary
- [x] 2.5 Add value totals to the statistics page

## 3. API and configuration

- [x] 3.1 Add the read-only pipeline resource to `internal/api`
- [x] 3.2 Add value fields to collection and game payloads
- [x] 3.3 Add `SHOP_CURRENCY` to `internal/config`, `.env.example` and the configuration docs
- [x] 3.4 Update `docs/api.md` and the OpenAPI document

## 4. Verification

- [x] 4.1 Table-driven tests for cost metrics covering no plays, no duration, no price, and mixed input
- [x] 4.2 Test that a second user's cost data never appears in a first user's response
- [x] 4.3 Test sort ordering with NULL values and confirm the query plan uses the play index
- [x] 4.4 Playwright test for the pipeline view and value sorting
- [x] 4.5 Run the axe checks on the pipeline and detail pages
