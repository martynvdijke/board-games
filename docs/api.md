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
| GET | `/loans` | read | `status=open\|closed\|all` (default `open`), `limit` 1–100 (default 24) for closed history |
| POST | `/loans` | write | Lend an owned game; `201`, or `409` when it is already out |
| GET | `/loans/{id}` | read | One loan; another user's loan id is `404`, never `403` |
| POST | `/loans/{id}/return` | write | Close an open loan; body and `returned_at` are optional |
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
      {"name": "Alex", "score": "42", "is_winner": true, "is_first_play": true},
      {"name": "Sam", "score": "38"}
    ],
    "notes": "Close one.",
    "client_play_id": "optional-idempotency-key"
  }'
```

`client_play_id` is optional and at most 128 characters. Send the same value
again to retry safely: the API answers `201` with the play created by the first
request rather than creating a second one.

BGG-imported plays (`origin: "bgg"`) cannot be edited or deleted through the
API; log a local correction instead.

### Loans

A loan records that an owned game is out with someone. At most one open loan
exists per game; dates are calendar days (`YYYY-MM-DD`), and `overdue`,
`days_out`, and `days_overdue` are derived by the server on every read.

Every loan has this shape (the embedded `game` is a reference, not the full
game resource):

```json
{
  "id": 1,
  "game": { "id": 9, "bgg_id": 1234, "name": "Patchwork", "thumbnail_url": "https://shelf.example.com/media/bgg/1234.jpg" },
  "borrower": "Sam",
  "contact": "",
  "lent_at": "2026-08-01",
  "due_at": "",
  "returned_at": "",
  "notes": "",
  "closed_reason": "",
  "overdue": false,
  "days_out": 101,
  "days_overdue": 0,
  "notified_at": ""
}
```

#### List loans

```sh
curl -H "Authorization: Bearer $TOKEN" \
  "https://shelf.example.com/api/v1/loans?status=all&limit=10"
```

```json
{ "loans": [] }
```

`open` (the default) lists open loans oldest lend date first; `closed` lists
recently returned loans newest return date first; `all` returns open loans
followed by closed history. `limit` bounds only the closed section. Numeric
values outside 1–100 are clamped; a non-numeric value is `400`.

#### Lend a game

```sh
curl -X POST https://shelf.example.com/api/v1/loans \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"game_id": 17, "borrower": "Sam", "due_at": "2026-12-01", "notes": "Brought to game night"}'
```

`lent_at` defaults to today. The game must be on your owned shelf and must not
already have an open loan. The response is `201` with the loan in `data`.

#### Mark a loan returned

```sh
curl -X POST https://shelf.example.com/api/v1/loans/1/return \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"returned_at": "2026-11-09"}'
```

The body is optional; an empty or omitted `returned_at` means today. The closed
loan is returned in `data` with `closed_reason: "returned"` and stays in
`status=closed` history.

#### Get one loan

```sh
curl -H "Authorization: Bearer $TOKEN" \
  https://shelf.example.com/api/v1/loans/1
```

Loan failures use the standard envelope:

| Status | When |
| --- | --- |
| `404` | The loan does not exist, or belongs to another user |
| `409` | The game already has an open loan; the message names the game |
| `422` | The game is not owned, the borrower is missing, or a date is malformed |

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

### Venues

`GET /venues` lists the places your plays happen at. A venue is created the
first time a play is logged with that location; spellings are matched
case-insensitively and with punctuation folded, so "The Crown" and "the crown"
share one venue. The spelling first seen is kept for display.

```json
{
  "data": {
    "venues": [
      {
        "id": 3,
        "name": "The Crown",
        "plays": 14,
        "distinct_games": 9,
        "last_played_at": "2026-10-04T19:30:00Z"
      }
    ]
  }
}
```

Ordered by play count, busiest first. Only the caller's venues are returned.
Requires the `read` scope.

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
