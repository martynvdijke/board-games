# REST API

Base path: `/api/v1`. Authentication is a personal token:

```sh
curl -H "Authorization: Bearer $TOKEN" https://shelf.example.com/api/v1/me
```

Tokens are created in **Settings → API tokens** and shown once. Scopes are
`read`, `write`, and `trmnl`. A `write` token also grants read access; a `trmnl`
token is intentionally limited to the feed.

The live OpenAPI 3.1 document is at `/api/openapi.json`; a browsable reference
is at `/api/docs`.

## Errors

All failures use one envelope:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The play could not be saved.",
    "fields": { "played_at": "Use YYYY-MM-DD." }
  }
}
```

Validation failures are `422`, malformed JSON or pagination are `400`,
unauthenticated is `401` with `WWW-Authenticate: Bearer`, missing/invisible
resources are `404`, conflicts such as an already-running sync are `409`, and
rate limiting is `429` with `Retry-After: 60`.

## Pagination

List endpoints take `limit` (1–100, default 24) and `offset` (≥0):

```json
{
  "data": [],
  "pagination": { "limit": 24, "offset": 0, "total": 132, "has_more": true }
}
```

## Endpoints

| Method | Path | Scope | Notes |
| --- | --- | --- | --- |
| GET | `/me` | read | Current user and token scopes |
| GET | `/me/export` | read | JSON copy of profile, settings, shelf, plays and token metadata |
| DELETE | `/me` | token or session+CSRF | Delete the account and all owned data; last admin gets `409` |
| DELETE | `/players?name=&mode=` | write | Remove (`remove`) or replace (`anonymise`) a player name; scoped to own plays unless admin |
| GET | `/games` | read | Filters: `q`, `players`, `max_playtime`, `min_weight`, `max_weight`, `category`, `mechanic`, `status`, `sort` |
| GET | `/games/{id}` | read | Full game, shelf state, setup notes, recent plays |
| PATCH | `/games/{id}` | write | Partial shelf update; send `null` to clear rating/price |
| GET | `/pipeline` | read | Acquisition buckets: wishlist, want to play, preordered, for trade, owned-unplayed; `limit` 1–96 per bucket |
| GET | `/plays` | read | Filters: `game_id`, `year`, pagination |
| POST | `/plays` | write | Local play with optional players |
| GET | `/plays/{id}` | read | Play and players |
| PATCH | `/plays/{id}` | write | Local plays only |
| DELETE | `/plays/{id}` | write | Local plays only |
| GET | `/stats` | read | Lifetime and yearly aggregates |
| GET | `/sync` | read | Latest and recent sync runs |
| POST | `/sync` | write | Starts a sync; `202` or `409` |
| GET | `/trmnl` | trmnl | 800×480 display payload |

### Start a sync

```sh
curl -X POST -H "Authorization: Bearer $TOKEN" \
  https://shelf.example.com/api/v1/sync
```

The response is `202` with the new run. Poll `GET /sync` or the web settings
page for progress. Triggering while a run is active returns `409`.

### Log a play

```sh
curl -X POST https://shelf.example.com/api/v1/plays \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "game_id": 17,
    "played_at": "2026-10-20",
    "location": "The kitchen table",
    "duration_minutes": 55,
    "players": [
      {"name": "Alex", "score": "42", "is_winner": true},
      {"name": "Sam", "score": "38"}
    ],
    "notes": "Close one."
  }'
```

BGG-imported plays (`origin: "bgg"`) cannot be edited or deleted through the
API; log a local correction instead.

### Shelf update

```sh
curl -X PATCH https://shelf.example.com/api/v1/games/17 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"favorite": true, "user_rating": 8.5, "location": "Living room"}'
```

Ratings use half steps from 1 to 10. `"user_rating": null` clears a rating.
`setup_notes` stores the private per-game setup guide (up to 5000 characters).

### Acquisition pipeline

```sh
curl -H "Authorization: Bearer $TOKEN" \
  "https://shelf.example.com/api/v1/pipeline?limit=12"
```

The response always contains all five states in this fixed order:

```json
{
  "buckets": [
    { "state": "wishlist", "label": "Wishlist", "count": 14, "games": [] },
    { "state": "want_to_play", "label": "Want to play", "count": 3, "games": [] },
    { "state": "preordered", "label": "Preordered", "count": 1, "games": [] },
    { "state": "for_trade", "label": "For trade", "count": 2, "games": [] },
    { "state": "unplayed", "label": "Owned, never played", "count": 9, "games": [] }
  ]
}
```

`count` is the total number of matching games and `games` is capped at `limit`
(1–96, default 24) per bucket. Buckets are derived from the existing
BoardGameGeek collection flags, so a game can appear in more than one bucket;
the states are not mutually exclusive. Only the requesting user's games are
returned.

### Value fields on games

Every game resource (`/games`, `/games/{id}`, `/pipeline` bucket entries, the
`PATCH` response) includes:

| Field | Type | Meaning |
| --- | --- | --- |
| `acquisition_date` | string or null | `YYYY-MM-DD`, or `null` when unset |
| `price_paid_cents` | integer or null | Price paid in integer minor units (pence/cents), or `null` |
| `price_per_play_cents` | number or null | `price_paid_cents` divided by recorded play quantity, rounded to 2 decimals |
| `price_per_hour_cents` | number or null | `price_paid_cents` per hour of recorded table time, rounded to 2 decimals |

The derived metrics are `null` — never `0` — when undefined:

- `price_paid_cents` is integer minor units, not a decimal amount.
- `price_per_play_cents` divides by recorded play quantity (`plays.quantity`);
  the BGG `num_plays` counter is not added to the denominator.
- `price_per_hour_cents` only counts plays that recorded a duration, so it is
  `null` when no play for that game has one, even if plays exist. In that case
  `price_per_play_cents` is still present.
- A game with no recorded price has all cost fields `null` and is excluded from
  value totals rather than treated as free.

### Collection value in statistics

`GET /stats` adds a `collection_value` object alongside the existing `shelf`
block:

```json
{
  "data": {
    "collection_value": {
      "total_paid_cents": 124500,
      "paid_games": 27,
      "unplayed_capital_cents": 31200,
      "median_cost_per_play_cents": 480
    }
  }
}
```

`total_paid_cents` sums the recorded price of owned games; `paid_games` counts
owned games with a price; `unplayed_capital_cents` is the recorded spend on
owned games with no recorded plays; `median_cost_per_play_cents` is `null` when
no owned game has both a price and plays. All amounts are integer minor units.

## TRMNL

```sh
curl -H "Authorization: Bearer $TRMNL_TOKEN" \
  https://shelf.example.com/api/v1/trmnl
```

Response shape:

```json
{
  "generated_at": "2026-10-23T08:00:00Z",
  "date": "2026-10-23",
  "stats": { "owned": 12, "unplayed": 4, "plays_this_year": 9 },
  "pick": { "game_id": 17, "name": "A Feast for Odin", "reason": "…", "cover_url": "https://…/media/bgg/109.jpg" },
  "recent": [],
  "shelf_of_shame": { "count": 4, "longest_waiting": "Pandemic Legacy", "longest_wait_days": 120 }
}
```

The pick is deterministic per user and date, using only owned, non-expansion
games with a preference for unplayed and long-waiting titles.
