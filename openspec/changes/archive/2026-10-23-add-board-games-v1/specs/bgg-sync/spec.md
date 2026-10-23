## ADDED Requirements

### Requirement: Users SHALL configure a public BoardGameGeek username

The settings page SHALL allow an authenticated user to save and validate a BoardGameGeek username. The system SHALL verify that the collection is readable without credentials and SHALL explain that private collections are unsupported. The username SHALL be stored per user.

#### Scenario: Save valid username
- **WHEN** a user saves a username whose BGG collection is public
- **THEN** the username SHALL persist and display on reload
- **AND** the user SHALL be able to start a sync

#### Scenario: Private or missing collection
- **WHEN** a user saves a username whose collection is private or does not exist
- **THEN** the system SHALL show a readable explanation that only public collections can be synced
- **AND** the username SHALL NOT be marked as syncable

#### Scenario: Empty username
- **WHEN** a user clears the username
- **THEN** scheduled sync SHALL be disabled for that user
- **AND** existing local data SHALL remain

### Requirement: Sync SHALL import owned games and collection state

A collection sync SHALL import every game in the user's BGG collection with its BGG identifier, name, year, image and thumbnail URLs, player counts, playtime, minimum age, personal rating, BGG play count, and collection status flags. Existing local edits to notes, acquisition details, location, and condition SHALL be preserved across syncs.

#### Scenario: First collection sync
- **WHEN** a user starts a sync with a public collection of 120 games
- **THEN** 120 game records and 120 user-shelf records SHALL be available locally
- **AND** owned, wishlist, preordered, and for-trade flags SHALL match BGG

#### Scenario: Local edits survive re-sync
- **WHEN** a user has written a note and recorded a price for a game
- **AND** a later sync updates that game's BGG metadata
- **THEN** the note and price SHALL remain unchanged

#### Scenario: Collection shrinks on BGG
- **WHEN** a game is removed from the BGG collection and a sync runs
- **THEN** the game SHALL no longer be marked owned for that user
- **AND** local plays and notes for the game SHALL be retained

### Requirement: Sync SHALL enrich games with BGG metadata

The system SHALL fetch game details in bounded batches and store the description, categories, mechanics, designers, publishers, weight, BGG rank, BGG rating, expansion flag, and image URLs. Metadata SHALL be refreshed when missing or older than the configured refresh window.

#### Scenario: Metadata enrichment
- **WHEN** a newly imported collection game has no stored description or categories
- **THEN** the sync SHALL fetch and store its BGG details
- **AND** the game SHALL become filterable by its categories and mechanics

#### Scenario: Batch limits
- **WHEN** many games need details
- **THEN** the system SHALL request them in batches rather than one request per game

#### Scenario: Partial metadata failure
- **WHEN** one detail batch fails
- **THEN** successfully stored batches SHALL remain committed
- **AND** the sync run SHALL report the failure and be resumable

### Requirement: Sync SHALL import play history idempotently

The system SHALL import the user's BGG plays with date, game, location, duration, quantity, notes, players, scores, and winner state. Each imported play SHALL be uniquely identified by the user and BGG play identifier so that repeated syncs do not create duplicates.

#### Scenario: First play import
- **WHEN** a user with 300 logged BGG plays syncs
- **THEN** all pages of play history SHALL be imported
- **AND** each play SHALL retain its players and scores

#### Scenario: Re-sync does not duplicate
- **WHEN** the same sync runs again
- **THEN** the play count SHALL remain unchanged
- **AND** no duplicate play rows SHALL exist

#### Scenario: New plays only
- **WHEN** new plays were logged on BGG since the previous sync
- **THEN** only those plays SHALL be added

#### Scenario: Pagination exhaustion
- **WHEN** the play history spans multiple pages
- **THEN** the sync SHALL continue until the final page is reached

### Requirement: Sync SHALL respect BoardGameGeek rate limits

The system SHALL limit outbound BGG requests to approximately one per second, send a descriptive `User-Agent` with contact information, and retry `202 Accepted`, `429 Too Many Requests`, and `503 Service Unavailable` responses with jittered backoff. A queued collection request SHALL be polled until it succeeds or the retry budget is exhausted.

#### Scenario: Queued collection response
- **WHEN** BGG responds `202 Accepted` for a collection request
- **THEN** the system SHALL wait and retry without reporting a fatal error

#### Scenario: Throttled response
- **WHEN** BGG responds `429` or `503`
- **THEN** the system SHALL back off and retry within the configured budget

#### Scenario: Retry budget exhausted
- **WHEN** retries do not succeed before the budget expires
- **THEN** the sync SHALL stop with a readable retryable error
- **AND** committed data SHALL NOT be rolled back

### Requirement: Sync progress and failures SHALL be visible

The system SHALL record each sync run with its phase, status, item counts, start and finish times, and an error message. The settings page SHALL show the current phase, progress, last successful sync, and the error when a run fails.

#### Scenario: Live progress
- **WHEN** a sync is running
- **THEN** the settings page SHALL show the current phase and completed item counts

#### Scenario: Failed sync
- **WHEN** a sync fails
- **THEN** the settings page SHALL show a plain-language error and a retry action
- **AND** the previous successful sync time SHALL remain visible

#### Scenario: Concurrent sync prevention
- **WHEN** a user starts a sync while one is already running for that user
- **THEN** the second request SHALL be rejected or joined to the running sync
- **AND** a second sync SHALL NOT run concurrently

### Requirement: Sync SHALL run on a schedule and on demand

Each user SHALL be able to enable or disable scheduled sync and choose an interval. A manual "Sync now" action SHALL always be available. Scheduled sync SHALL use the same service, limits, and status recording as manual sync.

#### Scenario: Scheduled sync
- **WHEN** the configured interval has elapsed and scheduled sync is enabled
- **THEN** a sync SHALL start automatically

#### Scenario: Scheduled sync disabled
- **WHEN** scheduled sync is disabled
- **THEN** no automatic sync SHALL start for that user
- **AND** manual sync SHALL remain available

### Requirement: BoardGameGeek SHALL be read-only

The system SHALL NOT store BoardGameGeek passwords or session cookies and SHALL NOT send write requests to BoardGameGeek. The interface SHALL state that collection and play changes are local.

#### Scenario: No credentials stored
- **WHEN** a user configures BGG sync
- **THEN** only the username SHALL be stored

#### Scenario: No write requests
- **WHEN** a user edits a rating, note, or play
- **THEN** no request that mutates BGG data SHALL be made
