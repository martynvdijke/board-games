## ADDED Requirements

### Requirement: Player linkage on play entry
Play records SHALL link each player row to a canonical player when one can be resolved.

#### Scenario: Creating a play with known players
- **WHEN** a play is created with player names that resolve to canonical players
- **THEN** each player row stores both the typed name and the canonical player reference

#### Scenario: Editing a play's players
- **WHEN** a user edits a local play and changes a player name
- **THEN** the player row is re-resolved and the previous canonical link is replaced

#### Scenario: Statistics by player
- **WHEN** play statistics are requested grouped by player
- **THEN** they aggregate by canonical player where linked, so spelling variants count as one person
