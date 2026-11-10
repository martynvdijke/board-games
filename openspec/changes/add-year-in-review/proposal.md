## Why

Every number a year-in-review needs already exists: plays, durations, prices,
ratings, first plays, and shelf-of-shame history. Assembling it is a read-only
aggregation, and the result is the only feature in this programme that markets
itself — people share recaps. It is November, so the window to ship it is now.

## What Changes

- Add a **year in review** page at `/year/{year}` computing from existing data:
  plays, distinct games, hours at the table, heaviest game played, most played
  game, biggest rating surprise, nemesis (most losses to one opponent),
  first plays, cost per hour, and shelf-of-shame progress against the same date
  a year earlier.
- Handle sparse and empty years honestly: a year with no plays renders a short,
  non-apologetic summary rather than a broken layout.
- Add an **opt-in public share**: a revocable, unguessable link showing the same
  recap with no account data, no notes, and no other users' names unless the
  user explicitly includes player labels.
- Keep the recap private by default; no third-party requests, no fonts or
  scripts from outside the instance, consistent with the existing privacy
  stance.
- Add navigation from statistics and from the annual rollover prompt in January.

### Non-goals

- Wrapped-style animated stories or video export.
- Multi-year comparisons beyond the shelf-of-shame delta.
- Social features, comments, or leaderboards across users.
- Push notifications announcing the recap.

## Capabilities

### New Capabilities

- `year-in-review`: Annual aggregation, the recap page, and the opt-in
  revocable public share.

## Modified Capabilities

- None.

## Impact

- `internal/plays` and `internal/collection`: annual aggregate queries, all
  parameterised and `user_id`-scoped.
- New migration: `year_review_shares` table (token, user, year, created,
  revoked, include_players).
- `internal/web/templates`: recap page and public share page.
- `internal/web`: public route that bypasses auth but not rate limiting.
- `web/src/style.css`: recap layout on existing tokens.
- No API surface in this change.
