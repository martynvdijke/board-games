## ADDED Requirements

### Requirement: Canonical players
The system SHALL maintain canonical player records per user, with alias resolution, while preserving the player name originally recorded on each play.

#### Scenario: Exact alias match links automatically
- **WHEN** a play is saved with a player name that matches an existing player's alias ignoring case
- **THEN** the play's player row is linked to that canonical player and the typed name is stored as recorded

#### Scenario: Unknown name offers a choice
- **WHEN** a play is saved with a name that matches no alias
- **THEN** the user is offered the choice to link it to an existing player or create a new one, and may also save it unlinked

#### Scenario: Unlinked players remain readable
- **WHEN** a play's player row has no canonical link
- **THEN** every list and statistic that displayed that name before continues to display it

#### Scenario: BoardGameGeek-imported name is unchanged
- **WHEN** a play is imported from BoardGameGeek
- **THEN** the stored player name is exactly the imported value and only the canonical link is added

### Requirement: Player autocomplete
Player inputs SHALL autocomplete from the user's own history.

#### Scenario: Suggestions are personal and recent
- **WHEN** a user begins typing a player name in the play form
- **THEN** suggestions come only from that user's players, ordered by recency of play, limited to ten

#### Scenario: No history
- **WHEN** the user has no players yet
- **THEN** the input accepts free text with no suggestions and no error

### Requirement: Player merge
The system SHALL allow a user to merge duplicate players into one.

#### Scenario: Merge rewrites links and keeps names
- **WHEN** a user merges player A into player B
- **THEN** all of A's play links point to B, A's aliases are added to B, A is removed, and the merge is recorded in an audit entry

#### Scenario: Merge is atomic
- **WHEN** a merge fails partway
- **THEN** no play links are changed and the players remain as before

#### Scenario: Duplicate suggestions
- **WHEN** a user views a player profile
- **THEN** likely duplicates are suggested from shared play dates and similar names, and merging remains an explicit action

### Requirement: Player profile
The system SHALL present a per-player statistics view.

#### Scenario: Profile content
- **WHEN** a user opens a player profile
- **THEN** it shows plays attended, overall win rate, win rate per game, head-to-head records, first plays witnessed, inferred preferred weight band, and last played date

#### Scenario: Head-to-head
- **WHEN** two players have played the same games together
- **THEN** the profile shows how often each won those games together

#### Scenario: Insufficient data
- **WHEN** a player has fewer than three plays
- **THEN** the profile shows the plays but omits rate-based sections rather than displaying misleading percentages
