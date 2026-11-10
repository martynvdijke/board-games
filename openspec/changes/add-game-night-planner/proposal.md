## Why

"On the table" answers *what is good*; it does not answer *what is good tonight
for four people with ninety minutes*. The pick ignores headcount, time budget,
appetite and freshness, which is exactly the information a group has at 19:30.
Fixing that turns the app's most opinionated feature into its most useful one.

## What Changes

- Add a **night planner**: given a headcount or named players, an available
  duration, a weight appetite, and a novelty preference, rank owned, playable
  games and explain each pick in one sentence.
- Enforce hard constraints: player count within the game's supported range, and
  estimated play time plus estimated teach time within the budget. Weight,
  freshness and novelty are soft preferences that shape ranking.
- **Infer teach and setup overhead per game** by comparing the user's own
  recorded `duration_minutes` with the BGG community play time. Show the
  observed average and the derived overhead; fall back to a conservative
  estimate when a game has fewer than three measured plays.
- **Infer the group's real weight tolerance** from the weight distribution of
  what the user actually plays, and offer it as a default appetite.
- Use freshness as a ranking signal: penalise games already played within a
  configurable window, and boost games never played or long dormant.
- When player profiles exist, prefer games that at least one attendee has never
  played, and say so.
- Let a chosen pick launch the game's setup guide directly, and save the plan so
  the resulting play can be logged against it.

### Non-goals

- Replacing the existing "on the table" pick or the TRMNL recommendation; they
  keep their current behaviour.
- Inventing play times when BGG data is missing and the user has no plays.
- Scheduling, invitations, or multiplayer negotiation.
- Machine-learned ranking; every rule stays explainable and inspectable.

## Capabilities

### New Capabilities

- `game-night-planner`: Constraint-based session planning, teach-time inference,
  weight-tolerance inference, and explainable ranking.

## Modified Capabilities

- None.

## Impact

- `internal/collection`: planner candidate query over `user_games` joined with
  `games`, respecting player counts, weight and ownership.
- `internal/plays`: duration aggregates per game and per weight band.
- New planner service with a pure ranking function that is unit-testable
  without a database.
- `internal/web/templates`: planner form, result list with explanations,
  link-through to setup guides.
- `internal/api`: read-only planner evaluation endpoint.
- `web/src`: small island for the interactive constraint form.
- `docs/api.md`, OpenAPI document, `README.md`.
- No schema migration; all inputs are derived from existing tables.
