## Context

The web app is server-rendered Go templates with small TypeScript islands, a warm
editorial design system, and strict accessibility expectations (WCAG AA, keyboard
access, reduced motion, visible focus). Plays are written through
`internal/plays` with players as rows in `play_players`.

Constraints: no client framework, no SPA router, CSRF on every state-changing
browser request, and assets imported through a Vite entry.

## Goals / Non-Goals

**Goals:**
- Remove the paper-and-retype step between the table and the play log.
- Be readable at arm's length and operable with a thumb.
- Never write a play until the user explicitly finishes.

**Non-Goals:**
- Multi-device live sync.
- Per-turn timers or rules adjudication.
- Score validation against game rules.

## Decisions

**The session is a server-owned draft with a client mirror.** A
`table_sessions` row holds game, attendees, mode, started_at and state. The island
keeps the live score state in `localStorage` and sends a compact snapshot on each
meaningful change (debounced), so a reload, a crash, or a dropped connection does
not lose the game in progress. The server is the source of truth on reload; the
local snapshot only wins when it is newer by a monotonic counter.

**Finishing is a single POST that converts the draft into a play.** The draft
carries duration from the session clock, per-player scores and winners; the
handler calls the existing play creation path inside one `db.WithTx`, then marks
the session finished and links the created play. Reusing the existing path means
CSRF, validation and player resolution behave identically to the normal form.

**Three scoring modes, chosen from the game, overridable per session.**
`competitive` (per-player scores, highest or lowest wins), `cooperative`
(win/loss for the table), `teams` (players grouped into teams, team score). The
default comes from the game's existing cooperative flag if present, otherwise
competitive, because guessing wrong is recoverable in one tap.

**Score entry is a numeric keypad island, not `input type=number`.** Native
number inputs are inconsistent across mobile keyboards and cannot be styled to
the design system. A custom keypad with `inputmode="numeric"` fallback and real
`<button>` elements keeps it accessible: each key is focusable, and each player's
total is announced via `aria-live` at coarse granularity.

**Round markers are optional and additive.** For games that score in rounds, a
"next round" action appends a column rather than replacing totals; the play's
notes receive a compact round summary on finish. Games that do not need rounds
never see the control.

**Layout is a dedicated template with large type, not a variant of the shelf.**
It uses the existing colour tokens at higher contrast, sizes targets to at least
48px, and offers a dim-room dark treatment. Landscape phone and portrait tablet
are the two target geometries; desktop centres the panel at a readable width.

**Drafts expire.** A session untouched for 24 hours becomes `abandoned` on the
next read, so a forgotten game does not linger in a half-written state.

## Risks / Trade-offs

- **Two sources of truth (server draft, local snapshot)** → the monotonic counter
  and a single merge rule, covered by unit tests for reload, offline edit, and
  double-tap finish.
- **A session left open in a pocket** → expiry plus an explicit discard control.
- **Custom keypad is an accessibility regression risk** → every key is a real
  button with a label, full keyboard support, visible focus, and a plain text
  fallback when JavaScript is unavailable.
- **Large-type layout fighting the design system** → derive all values from
  existing tokens and add no new palette; review with the same axe checks used
  elsewhere.

## Migration Plan

Add `table_sessions`. Deploy; the route is opt-in from the game page. Rollback
drops the table and the route; no plays are affected because finishing is the only
write to `plays`.

## Open Questions

- Should a finished session show a shareable result card image? Nice, but out of
  scope for the first version.
- Do we need a per-player turn order indicator? Useful for some games, but it
  risks becoming a rules feature; defer.
