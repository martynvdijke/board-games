## Why

Private setup guides already exist with up to twelve photos, but nothing routes
to them at the moment they are needed: someone is opening a box in front of
three people who have never played. The guide is a reference document; what the
table needs is a five-minute teach. Closing that gap is the cheapest way to make
the existing private content pay off.

## What Changes

- Extend the per-game guide with an ordered **teach script**: the hook, the goal,
  the turn, the end condition, and the three mistakes new players always make,
  each with an estimated duration in minutes.
- Add a **teach mode** view that presents the script one step at a time with a
  per-step timer and a total remaining time, usable full-screen on a phone or
  tablet propped against the box.
- Add a printable quick reference generated from the same script, with a print
  stylesheet that drops navigation and keeps the steps and photos legible in
  black and white.
- Add a QR code per game that resolves to the teach view, so a sticker on the
  box lid opens it; the code respects the user's session and does not leak a
  token.
- Offer "start teach" from the game detail page and from a planner result, with
  the measured teach time fed back so later estimates improve.

### Non-goals

- Authoring rules summaries from BGG content or any external source.
- Video, audio narration, or speech synthesis.
- Sharing teach scripts with other users or the public.
- Automatic rules extraction from PDFs.

## Capabilities

### New Capabilities

- `teach-mode`: Teach script authoring, stepwise teach presentation, printable
  quick reference, and per-game QR entry.

## Modified Capabilities

- None.

## Impact

- New migration: teach script steps table (game, user, position, title, body,
  minutes) or a JSON column on the existing guide record.
- `internal/web/templates`: teach view, print view, guide editor extension.
- `internal/web/static`: print stylesheet; QR generation server-side.
- `web/src`: timer island with reduced-motion and keyboard support.
- `internal/collection`: teach time aggregate read by the planner.
- `README.md`, `docs/api.md` if the guide shape is exposed.
