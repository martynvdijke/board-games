## Context

Setup guides are private per-game content with up to twelve photos, stored per
user and rendered inside the game detail page. The design system is warm
editorial with established tokens, WCAG AA contrast, keyboard access and
reduced-motion support. Assets must be imported through a Vite entry to be
hashed and embedded.

Constraints: server-rendered templates, small TypeScript islands only, no client
framework, and no external calls (fonts, CDNs, speech services) at runtime.

## Goals / Non-Goals

**Goals:**
- Turn a reference document into a teach flow usable while three people wait.
- Be legible on a phone propped against a box, and in black and white on paper.
- Feed measured teach time back into the planner.

**Non-Goals:**
- Authoring rules summaries from BGG or any external content.
- Video, audio, or narration.
- Sharing teach scripts between users.

## Decisions

**Teach steps are rows, not JSON.** A `guide_steps` table
(`user_id, game_id, position, title, body, minutes`) keeps ordering queryable,
lets the planner sum minutes in SQL, and avoids rewriting a blob on every edit.
Photos continue to reference the existing guide media.

**The teach view is a separate route, not a modal.** `/guide/{gameID}/teach`
renders a server-side page with one step per section and a small island for the
timer. A modal would trap focus, fight the browser back button, and print badly.

**The timer island is the only JavaScript.** It holds elapsed seconds in
`sessionStorage`, so a reload or an accidental swipe does not lose the teach.
It respects `prefers-reduced-motion`, uses `aria-live="polite"` for the remaining
time at coarse granularity (not per second, which would flood a screen reader),
and degrades to a static list with no JS.

**Printing is a stylesheet, not a PDF generator.** `@media print` drops
navigation, forces black on white, caps photo height, and breaks pages between
steps. This avoids a PDF dependency, works from any device, and lets the user
choose "Save as PDF" in the browser.

**QR codes are generated server-side for the game's teach URL.** The URL is
session-relative and requires sign-in, so a scanned code on a stranger's phone
shows a login page rather than content. The QR encodes the canonical absolute URL
built from `BASE_URL`, never a token, so a photographed sticker leaks nothing.

**Measured teach time is written back on finish.** When a teach run ends, the
elapsed minutes are stored on the guide (`last_teach_minutes`) and the planner
prefers that measurement over its inference for that user and game.

**Step ordering is a position integer with a unique index per guide.** Reordering
rewrites positions inside one `db.WithTx`; no drag-and-drop library.

## Risks / Trade-offs

- **Users will not author steps** → seed the editor from the existing free-text
  guide with a single "split into steps" action, and make step authoring optional
  so the old guide keeps working unchanged.
- **Print output varies by browser** → test the print stylesheet in Chromium and
  Firefox via Playwright `emulateMedia`, and keep photos optional in print.
- **QR pointing at an authenticated route is mildly confusing** → include the
  game name in the printed caption and a one-line instruction under the code.
- **Timer island adds a JS surface** → single file, no dependency, covered by a
  Vitest unit test for the elapsed-time formatter.

## Migration Plan

Add `guide_steps` and `last_teach_minutes`; existing guides render unchanged
because the teach route is only offered when steps exist. Backfill is a no-op.
Rollback drops the routes and leaves the tables inert.

## Open Questions

- Should the teach view show photos inline per step or in a fixed strip? Current
  preference: inline per step, with a fallback strip on very small screens.
- Does the printed sheet need the setup photos at all, or is that a separate
  "setup card" print? Leaning toward keeping photos off the teach sheet.
