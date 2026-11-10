## ADDED Requirements

### Requirement: CSV intake and mapping
The system SHALL accept a play export CSV, detect candidate columns, and let the user confirm the mapping before any data is written.

#### Scenario: Column detection
- **WHEN** a user uploads a CSV with recognisable headers
- **THEN** the system proposes a mapping for date, game, players, scores, winner, duration, location and notes, and the user can correct any of it

#### Scenario: Ambiguous dates
- **WHEN** the sampled dates could be read as either day-first or month-first
- **THEN** the user is required to choose a format before proceeding

#### Scenario: Unreadable dates
- **WHEN** some rows cannot be parsed with the chosen format
- **THEN** the preview lists those rows by line number and they are excluded from the import

#### Scenario: Missing required column
- **WHEN** no date or no game column can be identified
- **THEN** the import cannot proceed and the page explains which column is required

### Requirement: Dry run report
Before writing, the system SHALL report exactly what the import would do.

#### Scenario: Report content
- **WHEN** the mapping is confirmed
- **THEN** the preview states how many plays will be created, skipped as duplicates, or fail to match a game, and which players would be created or linked

#### Scenario: Unmatched games
- **WHEN** some rows match no game in the user's collection
- **THEN** they are listed with the source title so the user can resolve them, and they are not imported silently

#### Scenario: Nothing written
- **WHEN** the preview is displayed
- **THEN** no play or player has been created

### Requirement: Idempotent import
Importing the same export twice SHALL NOT duplicate plays.

#### Scenario: Repeated upload
- **WHEN** an export containing rows already imported is uploaded again
- **THEN** those rows are reported as duplicates and skipped

#### Scenario: Override
- **WHEN** a user explicitly chooses to import rows flagged as duplicates
- **THEN** those plays are created without an import key so the override is a deliberate one-time act

#### Scenario: Batch failure
- **WHEN** a batch of plays fails to commit
- **THEN** that batch is rolled back, the rest of the import continues, and the report identifies the failed batch

### Requirement: Imported plays are immutable
Imported plays SHALL be treated like imported BoardGameGeek plays.

#### Scenario: Editing an imported play
- **WHEN** a user attempts to edit or delete a play created by import
- **THEN** the change is refused with an explanation, as it is for BoardGameGeek-imported plays

#### Scenario: Subsequent sync
- **WHEN** a BoardGameGeek sync runs after an import
- **THEN** imported plays are neither modified nor removed by the sync
