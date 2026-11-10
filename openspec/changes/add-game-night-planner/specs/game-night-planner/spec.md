## ADDED Requirements

### Requirement: Session planning inputs
The system SHALL plan a game night from a headcount or named players, an available duration, a weight appetite, and a novelty preference, using only games the user owns.

#### Scenario: Plan from constraints
- **WHEN** a user submits four players, ninety minutes, a medium weight appetite and any novelty
- **THEN** the system returns ranked owned games that support four players and whose estimated play time plus estimated teach time fits within ninety minutes

#### Scenario: Hard constraints are respected
- **WHEN** a game supports three to five players but the requested headcount is six
- **THEN** the game never appears in the results, regardless of score

#### Scenario: No game fits
- **WHEN** no owned game satisfies the hard constraints
- **THEN** the page states that nothing fits and names the constraint that excluded the most games, rather than showing an unfiltered list

#### Scenario: Unowned games excluded
- **WHEN** a game is on the wishlist but not owned
- **THEN** it is never proposed as a playable option

### Requirement: Teach time inference
The system SHALL estimate per-game teach and setup overhead from the user's own recorded play durations, falling back to a conservative estimate when data is insufficient.

#### Scenario: Measured overhead
- **WHEN** a game has at least three plays with a recorded duration and a single copy played
- **THEN** the estimate uses the median recorded duration minus the published play time, clamped to a sane range, and the interface labels it as measured

#### Scenario: Insufficient plays
- **WHEN** a game has fewer than three measured plays
- **THEN** the estimate falls back to a weight-based default and the interface labels it as estimated

#### Scenario: Implausible duration
- **WHEN** a recorded duration implies an overhead outside the clamp range
- **THEN** the clamped value is used and the game is not excluded from planning

### Requirement: Weight tolerance inference
The system SHALL offer a default weight appetite inferred from the user's recent play history.

#### Scenario: Inferred default
- **WHEN** a user opens the planner and has played games in the last twelve months
- **THEN** the appetite control starts at a value derived from the heavy end of what they actually play, and the user can change it

#### Scenario: No recent plays
- **WHEN** the user has no plays in the last twelve months
- **THEN** the appetite control starts at a neutral default with no inference claimed

### Requirement: Explainable and deterministic ranking
Each result SHALL state why it was chosen, and the same inputs SHALL produce the same ordering.

#### Scenario: Explanation present
- **WHEN** a planner result is rendered
- **THEN** each game shows a short reason naming the fit and the strongest preference it satisfies

#### Scenario: Determinism
- **WHEN** the same constraints are evaluated twice with the same underlying data
- **THEN** the results appear in the same order, with ties broken by the game's sort name

#### Scenario: Freshness signal
- **WHEN** a game was played within the configured freshness window
- **THEN** it is penalised rather than removed, and the penalty is reflected in its explanation

#### Scenario: New to someone
- **WHEN** player profiles are available and at least one attendee has never played a candidate game
- **THEN** that game's ranking improves and its explanation says who is new to it
