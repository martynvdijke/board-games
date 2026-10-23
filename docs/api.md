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
| GET | `/games` | read | Filters: `q`, `players`, `max_playtime`, `min_weight`, `max_weight`, `category`, `mechanic`, `status`, `sort` |
| GET | `/games/{id}` | read | Full game, shelf state, recent plays |
| PATCH | `/games/{id}` | write | Partial shelf update; send `null` to clear rating/price |
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
