## ADDED Requirements

### Requirement: Imported play origin
The play origin SHALL distinguish locally created plays from plays imported from a file, and imported plays SHALL be immutable.

#### Scenario: Origin value
- **WHEN** a play is created by the import flow
- **THEN** its origin identifies it as imported and the interface marks it as not locally editable

#### Scenario: Immutability applies to every non-local origin
- **WHEN** a request attempts to edit or delete a play whose origin is not local
- **THEN** the request is refused regardless of whether the origin is BoardGameGeek or an imported file

#### Scenario: Player linking during import
- **WHEN** imported rows contain player names
- **THEN** those names are resolved to canonical players so imported history counts as one person
