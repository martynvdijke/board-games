## ADDED Requirements

### Requirement: Offline play queue
The mobile client SHALL allow a play to be composed and saved without connectivity and deliver it later exactly once.

#### Scenario: Logging offline
- **WHEN** a user saves a play while the API is unreachable
- **THEN** the play is stored locally with a client-generated identifier and appears in the plays list as pending

#### Scenario: Delivery on reconnect
- **WHEN** connectivity returns and a sync is triggered
- **THEN** queued plays are submitted and, on success, replaced by the server records

#### Scenario: Exactly once
- **WHEN** a queued play is submitted twice because a response was lost
- **THEN** the server returns the previously created play and no duplicate is created

#### Scenario: Retry exhaustion
- **WHEN** a queued play fails repeatedly past the retry limit
- **THEN** it is marked as needing attention, stops retrying automatically, and is visible to the user

#### Scenario: Queue after sign-out
- **WHEN** the user signs out while plays are queued
- **THEN** the queue is discarded and the user is warned beforehand

### Requirement: Sync visibility
The mobile client SHALL make unsynchronised data visible rather than silent.

#### Scenario: Indicator
- **WHEN** plays are pending or failed
- **THEN** the plays list shows an indicator and settings shows the pending count

#### Scenario: Conflict disclosure
- **WHEN** a queued play's response indicates the server record differs from what was queued
- **THEN** the server version is kept and the divergence is reported to the user once

### Requirement: Barcode game capture
The mobile client SHALL allow a game to be added by scanning its barcode.

#### Scenario: Successful scan
- **WHEN** a scanned code resolves to a game not already in the collection
- **THEN** the game's details are shown for confirmation and adding it puts it in the collection

#### Scenario: Unknown code
- **WHEN** a scanned code resolves to no game
- **THEN** the user is offered a text search instead of an error, and no record is created automatically

#### Scenario: Already owned
- **WHEN** a scanned code resolves to a game already owned
- **THEN** the user is told it is already on the shelf and offered to open it

#### Scenario: Camera unavailable or denied
- **WHEN** camera permission is denied or no camera is available
- **THEN** the capture entry point remains usable through manual search and does not block on a permission prompt at launch
