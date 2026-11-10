## Why

Scoring happens at the table, on paper or in someone's notes app, and is then
retyped into the play log — where it often never arrives. A screen designed to
sit on the table during a game removes that transcription step and is the most
visible, most shareable surface the app could have.

## What Changes

- Add a **table session** view: one game, the attending players, a running clock,
  per-player score entry with a large numeric keypad, round markers for games
  that score in rounds, and a winner determined by the game's scoring direction.
- Support cooperative games with win/loss and no per-player scores, and
  team-based games with a team grouping of players.
- Keep the input model thumb-friendly and glanceable at arm's length: large
  targets, high contrast, no dense chrome; the layout works landscape on a phone
  and portrait on a tablet.
- Start a session from the game detail page, from a planner result, or from a
  saved night plan; on finish, write a normal local play with duration taken
  from the session clock and scores from the session, then show the result.
- Preserve the session across a page reload and a dropped connection by storing
  drafts locally, and never write a play until the user finishes.

### Non-goals

- Real-time multi-device sync of a live session.
- Timers enforced per turn, or rules adjudication.
- Score validation against game rules.
- A separate native app; this is a server-rendered page with a small island.

## Capabilities

### New Capabilities

- `table-session`: Session draft lifecycle, score and clock capture, cooperative
  and team modes, and conversion into a play record.

## Modified Capabilities

- None.

## Impact

- New migration: `table_sessions` draft table scoped by `user_id`, with a
  finished/abandoned state and a link to the created play.
- `internal/plays`: creation path reused; no change to play semantics.
- `internal/web/templates`: session page and finish summary.
- `web/src`: score entry and clock island; must survive reload via local draft.
- `web/src/style.css`: large-type session layout derived from existing tokens,
  with a dark high-contrast variant for dim rooms.
- Accessibility: every score control reachable by keyboard, announced values.
