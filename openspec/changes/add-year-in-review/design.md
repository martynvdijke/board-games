## Context

Plays, durations, prices, ratings, first-play flags and play timestamps are all
present. Statistics already aggregate per user. The app has a strict privacy
posture: no third-party requests, opt-in analytics, EU-facing legal pages, and
user-scoped queries that answer 404 rather than leaking existence.

Constraints: server-rendered pages, no client framework, and any public surface
must be explicitly opt-in and revocable.

## Goals / Non-Goals

**Goals:**
- A recap that is genuinely personal and correct for a sparse year.
- A public share that leaks nothing the user did not choose to publish.
- No new third-party dependency, font, or script.

**Non-Goals:**
- Animated stories or video export.
- Cross-user leaderboards.
- Automatic annual emails or notifications.

## Decisions

**Compute in SQL, assemble in Go, render with one template per section.**
Sections be overlaid as partials so a year with no plays can skip sections
without a conditional thicket. Every query is parameterised and `user_id`
scoped; the year boundary is passed as two RFC3339 strings from `db.Now()`-style
helpers, not SQLite date functions, so timezone rules stay in one place.

**"Biggest surprise" is defined, not vibes.** The game whose user rating sits
furthest above the median rating of games first played that year, with a minimum
of three first plays before the section renders at all. Publishing an undefined
superlative is worse than omitting it.

**Shelf-of-shame progress compares to the same calendar date a year earlier.**
Recompute "unplayed owned games as of date D" from `acquisition_date` where
present, and fall back to `added_at`. Document the fallback on the page as a
footnote rather than pretending precision.

**Sharable recap is a token row, not a signed URL.** A `year_review_shares` row
holds an opaque 128-bit token, the user, the year, `include_players`, and a
nullable `revoked_at`. Revocation is a row update, so links die without rotating
secrets. Tokens are compared in constant time and are not enumerable.

**Player names are excluded unless `include_players` is set.** The public page
then shows "your nemesis" rather than a name. This is the single most likely
accidental disclosure, so it is a column, not a template flag.

**The public page is rate limited and `noindex`.** It carries no session, no
CSRF token, and no navigation into the private app. It uses the same stylesheet
so it looks like the product, with a footer linking to the instance.

**Empty years return 404 from `/year/{year}`** when the year is before the
user's first play or after the current year, rather than rendering a hollow page.

## Risks / Trade-offs

- **Aggregation cost** → each section is one indexed aggregate over that user's
  plays; cache the rendered page for a short window keyed by user and year, and
  invalidate on play write. Acceptable to skip caching initially.
- **Accidental disclosure via share link** → default `include_players = false`,
  show a preview of exactly what will be public before creating the link.
- **Users sharing a link and forgetting** → shares are listed in settings with a
  revoke button and a created-at date.
- **Timezone drift at year boundaries** → use the instance's configured
  timezone consistently for both the query and the display, and test a play at
  23:59 on 31 December.

## Migration Plan

Add `year_review_shares`. Deploy; the feature is read-only until a user creates
a share. Rollback drops the table and the routes.

## Open Questions

- Is an annual "your recap is ready" banner wanted, or is discoverability from
  the statistics page enough?
- Should the recap be printable as a one-page PDF via the same print stylesheet
  as teach mode? Likely yes, but it can follow.
