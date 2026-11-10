## ADDED Requirements

### Requirement: Group shelf scope
The shelf SHALL support a group scope in addition to the per-user scope.

#### Scenario: Switching scope
- **WHEN** a member of at least one group views the shelf
- **THEN** they can switch between their own shelf and a group shelf without losing filter state that still applies

#### Scenario: Owner facet
- **WHEN** the shelf is in group scope
- **THEN** an owner facet is available alongside the existing filters

#### Scenario: Personal fields in group scope
- **WHEN** a game is shown from another member's shelf
- **THEN** private personal fields such as notes, price paid and setup guides are not shown

#### Scenario: Default scope unchanged
- **WHEN** a user belongs to no group, or has never opened a group shelf
- **THEN** the shelf behaves exactly as before
