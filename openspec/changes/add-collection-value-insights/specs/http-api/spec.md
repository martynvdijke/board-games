## ADDED Requirements

### Requirement: Value and pipeline API resources
The API SHALL expose the acquisition pipeline and per-game value fields.

#### Scenario: Pipeline resource
- **WHEN** an authenticated client requests the pipeline resource
- **THEN** the response contains each state with a count and a paginated game list scoped to the requesting user

#### Scenario: Value fields on games
- **WHEN** an authenticated client requests a collection or game resource
- **THEN** the response includes price paid, acquisition date, and the computed cost per play and cost per hour, using null where undefined

#### Scenario: Scoping
- **WHEN** a token requests the pipeline resource
- **THEN** only the token owner's games are returned and no other user's games are reachable through any parameter
