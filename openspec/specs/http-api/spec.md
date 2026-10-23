# http-api Specification

## Purpose
TBD - created by archiving change add-board-games-v1. Update Purpose after archive.
## Requirements
### Requirement: The system SHALL expose a versioned JSON API

The API SHALL be served under `/api/v1`, accept and return JSON encoded as UTF-8, and use plural resource names. Every response SHALL include a JSON content type and SHALL support authenticated access through the mechanisms defined by the authentication capability.

#### Scenario: Versioned endpoint
- **WHEN** a client calls a valid `/api/v1` endpoint with a valid credential
- **THEN** the response SHALL be JSON with an appropriate `2xx` status

#### Scenario: Unknown endpoint
- **WHEN** a client calls an unknown API path
- **THEN** the response SHALL be `404 Not Found` with the standard error envelope

### Requirement: The API SHALL expose collection resources

The API SHALL provide endpoints to read the current user, list the shelf, read a single game with its collection state and metadata, and update personal collection state. Listing SHALL support the same search, filter, sort, and pagination semantics as the web shelf.

#### Scenario: List games
- **WHEN** a client requests `GET /api/v1/games?q=catan&owned=true`
- **THEN** the response SHALL contain matching games and collection state for the authenticated user
- **AND** it SHALL include pagination metadata

#### Scenario: Read a game
- **WHEN** a client requests `GET /api/v1/games/{id}` for a game on the user's shelf
- **THEN** the response SHALL include metadata, collection state, and recent plays

#### Scenario: Update collection state
- **WHEN** a client sends `PATCH /api/v1/games/{id}` with `favorite`, `rating`, or `notes`
- **THEN** only the supplied fields SHALL change
- **AND** the updated collection state SHALL be returned

#### Scenario: Invalid game
- **WHEN** a client requests a game that does not exist or is not on the user's shelf
- **THEN** the API SHALL return `404 Not Found`

### Requirement: The API SHALL expose play resources

The API SHALL provide endpoints to list, create, update, and delete plays for the authenticated user, including players and scores. Play payloads SHALL be validated with the same rules as the web interface.

#### Scenario: Create a play
- **WHEN** a client posts a valid play with players
- **THEN** the API SHALL return `201 Created` with the stored play and its identifier

#### Scenario: Update a play
- **WHEN** a client patches an owned play
- **THEN** the API SHALL return the updated play

#### Scenario: Delete a play
- **WHEN** a client deletes an owned play
- **THEN** the API SHALL return `204 No Content`

#### Scenario: Invalid payload
- **WHEN** a client posts a play with an invalid date or unknown game
- **THEN** the API SHALL return `422 Unprocessable Entity` with field-level details
- **AND** no play SHALL be created

### Requirement: The API SHALL expose sync and statistics resources

The API SHALL allow a client to start a BGG sync, read sync status and history, and read the user's statistics summary.

#### Scenario: Start sync
- **WHEN** a client posts to the sync endpoint and no sync is running
- **THEN** the API SHALL return `202 Accepted` with a sync run identifier

#### Scenario: Sync already running
- **WHEN** a client starts a sync while one is running for that user
- **THEN** the API SHALL return `409 Conflict` with the active run status

#### Scenario: Read statistics
- **WHEN** a client requests the statistics endpoint
- **THEN** the response SHALL contain owned, unplayed, plays-this-year, and play aggregate values for that user

### Requirement: The API SHALL paginate and report totals

Collection and play list endpoints SHALL accept `limit` and `offset`, default to a documented page size, cap the maximum page size, and return the total matching count and whether more results exist.

#### Scenario: Default pagination
- **WHEN** a client lists games without `limit` or `offset`
- **THEN** the response SHALL use the default page size and `offset` zero

#### Scenario: Maximum page size
- **WHEN** a client requests a `limit` above the cap
- **THEN** the server SHALL clamp it to the cap or return a validation error with the allowed maximum

#### Scenario: Invalid pagination
- **WHEN** a client sends a negative `limit` or `offset`
- **THEN** the API SHALL return `400 Bad Request`

### Requirement: Errors SHALL use a consistent envelope

The API SHALL return errors as `{"error":{"code":"<stable-code>","message":"<human message>","fields":{...}}}`. It SHALL NOT include stack traces, SQL, tokens, or internal file paths.

#### Scenario: Validation error
- **WHEN** a request fails validation
- **THEN** the response SHALL use the error envelope with a stable code and field details

#### Scenario: Authentication error
- **WHEN** a request is unauthenticated or the token is invalid
- **THEN** the response SHALL be `401 Unauthorized` with the error envelope
- **AND** it SHALL include a `WWW-Authenticate` header

#### Scenario: Internal error
- **WHEN** an unexpected server error occurs
- **THEN** the response SHALL be `500 Internal Server Error` with a generic message
- **AND** the detailed cause SHALL be logged server-side with a correlation identifier

### Requirement: The API SHALL be documented with OpenAPI

The system SHALL publish an OpenAPI 3 document describing every `/api/v1` endpoint, its authentication and scopes, parameters, request bodies, responses, and the error envelope. A browsable documentation page SHALL be available, and the document SHALL be fetchable as JSON.

#### Scenario: Fetch specification
- **WHEN** a developer requests the OpenAPI JSON
- **THEN** the response SHALL be a valid OpenAPI 3 document covering every public API route

#### Scenario: Browsable docs
- **WHEN** a developer opens the documentation page
- **THEN** the endpoints SHALL be listed with authentication requirements and example payloads

### Requirement: The API SHALL be rate limited

Authenticated API requests SHALL be rate limited per token or user, returning `429 Too Many Requests` with a `Retry-After` header when the limit is exceeded. Authentication failures SHALL be limited per source address.

#### Scenario: Exceeded limit
- **WHEN** a client exceeds the configured request rate
- **THEN** the API SHALL return `429 Too Many Requests`
- **AND** include a `Retry-After` header

#### Scenario: Brute-force protection
- **WHEN** a source repeatedly sends invalid credentials
- **THEN** further attempts from that source SHALL be delayed or rejected for a cool-down period

