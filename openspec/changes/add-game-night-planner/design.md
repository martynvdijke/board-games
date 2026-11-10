## Context

Games carry BGG metadata: `min_players`, `max_players`, `playing_time`,
`weight`. `user_games` carries ownership and personal state. Plays carry
`duration_minutes` and `quantity`, and `play_players` carries names. The existing
"on the table" pick is a separate, stable, cache-friendly recommendation used by
the TRMNL feed.

Constraints: the ranking must be explainable, deterministic for the same inputs,
fast enough for a synchronous page render, and entirely derived from existing
tables — no new columns.

## Goals / Non-Goals

**Goals:**
- Answer "what should we play tonight" with hard constraints and honest reasons.
- Learn teach overhead from the user's own data instead of guessing.
- Keep the ranking function pure and unit-testable without a database.

**Non-Goals:**
- Replacing the existing pick or the TRMNL feed.
- Learning a model; every weight is a named constant.
- Negotiating between attendees.

## Decisions

**Two-phase ranking: hard filter, then weighted score.** Hard constraints are
player count fit and time fit. Everything else — weight, freshness, novelty,
whether someone is new to the game — is a scored preference. This keeps the
explanation honest: "fits 4 players in 90 minutes" is a fact, "you have not
played it in 5 months" is a preference.

**Teach overhead is inferred as a robust difference, not a raw average
subtraction.** For each game, take the median of the user's `duration_minutes`
for plays with `quantity = 1`, subtract BGG's `playing_time`, clamp to a sane
range (0 to 45 minutes), and require at least three measured plays. Below that
threshold, use a conservative default derived from weight (light 5, medium 10,
heavy 20). Medians resist the one four-hour Groceries-and-Gunplay evening that
would otherwise poison the estimate.

**Weight tolerance is inferred from the plays the user actually logs, weighted
by recency.** Compute the 90th percentile BGG weight of games played in the last
twelve months; offer that as the default appetite, and let the user raise it for
one night. This is a default, not a gate — the UI always shows the slider.

**Freshness uses recency of play plus a novelty mode.** Three modes: *anything*,
*nothing played in N days* (default 30), *new to the table* (never played, or
never played by at least one attendee when profiles exist). Freshness is a
penalty term, so a perfect-fit stale game can still win.

**The scoring function lives in `internal/collection` as a pure function over a
candidate struct.** No SQL, no clock, no randomness: the time and the recency
window are inputs. Tests enumerate the weights and assert ordering and
explanations, which is what makes the feature trustworthy.

**Determinism is per-input, but the result list is not shuffled.** Ties break on
`sort_name`, so two people running the same query see the same top three. A
"surprise me" affordance can sample from the top five later without changing the
core.

**One round trip for candidates.** A single query returns owned games with
player counts, weight, BGG play time and a correlated subquery for the last
played date and play count; the Go layer does the rest. Avoid N+1 lookups per
candidate.

## Risks / Trade-offs

- **BGG `playing_time` is often wrong** → the inferred overhead compensates over
  time, and the UI states when an estimate is BGG-only versus measured.
- **Sparse data makes inference weak** → explicit fallbacks with a visible
  "estimated" label; never present a guess as a measurement.
- **Too many knobs** → four inputs only (people, minutes, appetite, novelty),
  with inferred defaults so a user can press one button.
- **Planner and "on the table" diverge confusingly** → cross-link them and state
  the difference in one sentence on each page.

## Migration Plan

No schema change. Ship behind the same navigation as "on the table"; rollback is
removing the route.

## Open Questions

- Should a saved plan be a first-class object that a play can reference, or is
  "open the planner, then log a play" enough? Start with the latter.
- When player profiles ship, does "new to the table" gate results or only score
  them? Preference: score, never gate, so the list is never empty.
