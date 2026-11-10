## ADDED Requirements

### Requirement: Annual recap
The system SHALL render a per-user recap for a given calendar year from existing collection and play data.

#### Scenario: Recap content
- **WHEN** a signed-in user opens the recap for a year in which they logged plays
- **THEN** it shows plays, distinct games, hours at the table, most played game, heaviest game played, first plays, cost per hour, nemesis, biggest rating surprise, and shelf-of-shame progress against the same date one year earlier

#### Scenario: Only the user's data
- **WHEN** a recap is rendered
- **THEN** every figure derives only from the requesting user's plays and collection, and no other user's data is read

#### Scenario: Superlative with insufficient data
- **WHEN** the year has fewer than three first plays
- **THEN** the biggest-surprise section is omitted rather than showing an arbitrary game

#### Scenario: Invalid year
- **WHEN** a year is requested that is after the current year or before the user's first play
- **THEN** the route returns not found instead of an empty recap

### Requirement: Sparse years
A recap SHALL render honestly when a year has little or no activity.

#### Scenario: No plays
- **WHEN** the year has no plays but the user existed
- **THEN** the recap shows a short summary acknowledging the gap and still shows shelf-of-shame state, without empty sections or broken layout

#### Scenario: Missing durations
- **WHEN** some plays lack durations
- **THEN** hours at the table are computed from plays that have a duration and the page notes the basis

### Requirement: Opt-in public share
A user SHALL be able to publish a recap through a revocable link that exposes only what they choose.

#### Scenario: Share is off by default
- **WHEN** a user has never created a share
- **THEN** the recap is reachable only to that signed-in user and no public URL exists

#### Scenario: Creating a share
- **WHEN** a user creates a share for a year
- **THEN** a preview shows exactly what becomes public before the link is generated, and the resulting token is unguessable

#### Scenario: Player names excluded by default
- **WHEN** a share is created without choosing to include player labels
- **THEN** the public page refers to opponents generically and contains no player names

#### Scenario: Revoking a share
- **WHEN** a user revokes a share
- **THEN** the public URL immediately returns not found and the recap stays available to the owner

#### Scenario: Public page isolation
- **WHEN** the public recap page is served
- **THEN** it sets no session cookie, contains no navigation into the private application, is marked noindex, and makes no third-party requests
