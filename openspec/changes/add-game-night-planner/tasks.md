## 1. Candidate data

- [ ] 1.1 Add a single candidate query returning owned games with player counts, weight, published play time, play count and last played date
- [ ] 1.2 Add a per-game duration aggregate used for teach inference
- [ ] 1.3 Add a recent-play weight distribution query for tolerance inference
- [ ] 1.4 Exclude unowned, wishlist-only and preordered-only games from candidates

## 2. Inference

- [ ] 2.1 Implement teach-overhead inference using the median measured duration minus published play time, clamped, with a three-play threshold
- [ ] 2.2 Implement the weight-based fallback and mark estimates as estimated rather than measured
- [ ] 2.3 Implement weight-tolerance inference from the 90th percentile of weights played in the last twelve months
- [ ] 2.4 Unit-test both inference functions on empty, sparse and implausible inputs

## 3. Ranking

- [ ] 3.1 Define the candidate and constraint structs and the pure scoring function with named weights
- [ ] 3.2 Apply hard filters for player count and for play time plus teach time against the budget
- [ ] 3.3 Add freshness penalties for the three novelty modes and a determinism tie-break on sort name
- [ ] 3.4 Add the "new to someone" bonus when player profiles resolve attendees
- [ ] 3.5 Generate the one-sentence explanation per result from the applied factors

## 4. Interface

- [ ] 4.1 Add the planner route and form with headcount, duration, appetite and novelty, defaulted from inference
- [ ] 4.2 Render ranked results with explanations and a link to each game's setup guide
- [ ] 4.3 Add the empty-result state naming the most restrictive constraint
- [ ] 4.4 Add a TypeScript island for the constraint form only
- [ ] 4.5 Cross-link the planner and the existing pick, explaining the difference in one sentence

## 5. API and verification

- [ ] 5.1 Add a read-only evaluation endpoint and update `docs/api.md` with the OpenAPI document
- [ ] 5.2 Test that hard constraints are never violated across a matrix of headcounts and durations
- [ ] 5.3 Test determinism by evaluating the same constraints twice
- [ ] 5.4 Test that an unowned game never appears as a candidate
- [ ] 5.5 Playwright test for the planner flow from form to setup guide
