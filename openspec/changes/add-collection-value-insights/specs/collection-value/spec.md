## ADDED Requirements

### Requirement: Acquisition pipeline
The system SHALL present a pipeline view grouping a user's games by the acquisition states already imported from BoardGameGeek.

#### Scenario: Pipeline shows each state
- **WHEN** a signed-in user opens the pipeline view
- **THEN** the page shows separate buckets for wishlist, want to play, preordered, for trade, and owned-not-yet-played, each with a count and the matching games

#### Scenario: A game belongs to more than one state
- **WHEN** a game is both preordered and on the wishlist
- **THEN** it appears in both buckets and the page does not imply the states are mutually exclusive

#### Scenario: Empty state
- **WHEN** the user has no games in any acquisition state
- **THEN** the page explains that states come from the BoardGameGeek collection and that a sync will fill them, without showing empty headings for every bucket

### Requirement: Cost per use
The system SHALL report the price paid per play and per hour of table time for each owned game, derived from the recorded price and the user's own plays.

#### Scenario: Game with plays and duration
- **WHEN** a game has a recorded price and at least one play with a recorded duration
- **THEN** the game's detail page and the collection list show a price per play and a price per hour

#### Scenario: Game with no plays
- **WHEN** a game has a recorded price but no plays
- **THEN** the cost per play is shown as undefined rather than zero, and it sorts last when sorting by cost

#### Scenario: Play without duration
- **WHEN** plays exist but none record a duration
- **THEN** price per play is shown and price per hour is shown as undefined

#### Scenario: No price recorded
- **WHEN** a game has no recorded price
- **THEN** no cost metric is shown and the game is excluded from value totals rather than counted as free

### Requirement: Trade candidates
The system SHALL rank owned games as trade candidates using an explicit, stated rule and SHALL explain why each candidate ranked.

#### Scenario: Candidate list
- **WHEN** a user opens the trade candidate list
- **THEN** owned games are ranked by unplayed status, time since last play, price paid relative to the collection, user rating, and the for-trade flag, and each row states the reason for its rank

#### Scenario: Nothing qualifies
- **WHEN** no owned game meets the minimum candidacy criteria
- **THEN** the page says so explicitly instead of showing an empty ranking

#### Scenario: A candidate leaves the shelf
- **WHEN** a game is no longer owned after a sync
- **THEN** it no longer appears as a trade candidate

### Requirement: Collection value totals
The system SHALL include value totals in collection statistics.

#### Scenario: Totals are present
- **WHEN** a user with recorded prices opens statistics
- **THEN** the page shows total recorded spend, median cost per play, and the total paid for owned games with no plays

#### Scenario: Currency
- **WHEN** value figures are displayed
- **THEN** they use the instance-configured currency code and are formatted for that currency without per-game currency selection
