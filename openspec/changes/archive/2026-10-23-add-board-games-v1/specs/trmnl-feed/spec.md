## ADDED Requirements

### Requirement: The TRMNL feed SHALL require a scoped token

The TRMNL endpoint SHALL require a valid personal API token with the `trmnl` scope, supplied as a bearer token or a query parameter. Requests without a valid token SHALL be rejected without revealing shelf data.

#### Scenario: Valid token
- **WHEN** a TRMNL device polls the feed with a valid `trmnl` token
- **THEN** the request SHALL return the user's e-ink payload

#### Scenario: Missing token
- **WHEN** a request reaches the TRMNL endpoint without a token
- **THEN** the response SHALL be `401 Unauthorized`
- **AND** no collection data SHALL be included

#### Scenario: Revoked token
- **WHEN** a device polls with a revoked token
- **THEN** the response SHALL be `401 Unauthorized`

### Requirement: The feed SHALL provide a compact e-ink payload

The payload SHALL include collection statistics (owned, unplayed, plays this year, wishlist), one game recommendation, a short recent list, and a shelf-of-shame summary. It SHALL be small enough for an e-ink device to poll frequently and SHALL include a generation timestamp.

#### Scenario: Populated feed
- **WHEN** the user has a synced shelf and plays
- **THEN** the payload SHALL contain accurate statistics, a recommendation, recent games, and the shelf-of-shame count

#### Scenario: Empty shelf
- **WHEN** the user has no games
- **THEN** the feed SHALL return a valid payload with zeroed statistics and a null recommendation
- **AND** it SHALL NOT return a server error

### Requirement: The recommendation SHALL be stable and explainable

The recommended game SHALL be deterministic for a given user and local calendar day, so repeated polls return the same game. It SHALL favour owned, non-expansion games and consider supported player count, playtime, unplayed status, and personal rating. The payload SHALL include a short human-readable reason for the choice.

#### Scenario: Repeated polls
- **WHEN** a device polls twice on the same day
- **THEN** the same game SHALL be recommended both times

#### Scenario: New day
- **WHEN** the local calendar day changes
- **THEN** the recommendation MAY change

#### Scenario: Reason present
- **WHEN** a recommendation is returned
- **THEN** it SHALL include a concise reason such as being unplayed or fitting a weeknight

#### Scenario: No eligible game
- **WHEN** no owned non-expansion game exists
- **THEN** the recommendation SHALL be null
- **AND** the payload SHALL remain valid

### Requirement: Cover images SHALL be device-accessible

Any image URL in the payload SHALL be absolute, use the configured public base URL, and be served by the application rather than hotlinked from BoardGameGeek. The image SHALL remain available when the device polls without cookies.

#### Scenario: Absolute image URL
- **WHEN** a recommendation includes a cover
- **THEN** the URL SHALL start with the configured `BASE_URL`

#### Scenario: Image proxy fallback
- **WHEN** the original BGG image cannot be fetched
- **THEN** the application SHALL serve a deterministic placeholder image
- **AND** the payload SHALL still include an image URL

### Requirement: The system SHALL provide a TRMNL preview

The application SHALL render an HTML preview of the e-ink payload at an 800×480 aspect ratio so the user can tune their device layout. The preview SHALL use the same data and recommendation as the JSON feed.

#### Scenario: Preview available
- **WHEN** an authenticated user opens the TRMNL preview
- **THEN** the page SHALL show the same recommendation and statistics as the feed in an e-ink-style layout

#### Scenario: Preview requires authentication
- **WHEN** an unauthenticated visitor opens the preview
- **THEN** the visitor SHALL be redirected to login

### Requirement: The feed SHALL be cache-friendly

The feed SHALL return a JSON content type and a cache policy appropriate for polling devices. Responses SHALL include the generation time and SHALL not require cookies or JavaScript.

#### Scenario: Cache headers
- **WHEN** a device polls the feed
- **THEN** the response SHALL include a short cache lifetime and JSON content type

#### Scenario: No JavaScript requirement
- **WHEN** a device with no JavaScript support requests the feed
- **THEN** the full payload SHALL be returned
