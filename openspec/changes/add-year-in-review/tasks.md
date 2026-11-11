## 1. Aggregates

- [x] 1.1 Add year-bounded play aggregates: plays, distinct games, summed duration, quantity
- [x] 1.2 Add most played, heaviest played, and first plays for the year
- [x] 1.3 Add the biggest-surprise calculation with the three-first-play threshold
- [x] 1.4 Add nemesis detection from head-to-head results within the year
- [x] 1.5 Add shelf-of-shame position at a given date, using acquisition date with an `added_at` fallback
- [x] 1.6 Add cost per hour for the year from recorded prices and durations

## 2. Recap page

- [x] 2.1 Add the `/year/{year}` route with year validation and not-found for out-of-range years
- [x] 2.2 Build the recap template from section partials, skipping sections without data
- [x] 2.3 Implement the sparse-year variant with an honest short summary
- [x] 2.4 Link the recap from the statistics page
- [x] 2.5 Add recap styles using existing design tokens only

## 3. Public share

- [x] 3.1 Add a migration for `year_review_shares` with a token, year, owner, `include_players` and `revoked_at`
- [x] 3.2 Add create and revoke actions in settings, listing existing shares
- [x] 3.3 Show a preview of the public page before the link is created
- [x] 3.4 Add the unauthenticated public route with constant-time token comparison, `noindex`, no session cookie and rate limiting
- [x] 3.5 Exclude player names unless the share opts in

## 4. Verification

- [x] 4.1 Test the biggest-surprise threshold and its omission below three first plays
- [x] 4.2 Test that a year with no plays renders the sparse variant without empty sections
- [x] 4.3 Test that a revoked token returns not found immediately
- [x] 4.4 Test that the public page contains no player names when the share excludes them
- [x] 4.5 Test year boundaries with a play at 23:59 on 31 December in the instance timezone
- [x] 4.6 Run axe checks on the recap and public pages
