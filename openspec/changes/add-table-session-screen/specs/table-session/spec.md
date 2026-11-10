## ADDED Requirements

### Requirement: Session draft lifecycle
The system SHALL keep a table session as a draft until the user explicitly finishes or discards it, and SHALL write no play before that point.

#### Scenario: Starting a session
- **WHEN** a user starts a session for a game with named attendees
- **THEN** a draft is created with the game, attendees, mode and start time, and no play exists yet

#### Scenario: Resuming after a reload
- **WHEN** the session page is reloaded
- **THEN** the current scores, attendees and elapsed time are restored from the server draft or the newer local snapshot

#### Scenario: Finishing writes one play
- **WHEN** the user finishes a session
- **THEN** exactly one play is created with the session's duration and scores, the draft is marked finished, and the created play is linked to it

#### Scenario: Discarding
- **WHEN** the user discards a session
- **THEN** the draft is marked abandoned and no play is created

#### Scenario: Stale drafts
- **WHEN** a draft has been untouched for twenty-four hours
- **THEN** it is treated as abandoned on the next read and does not appear as active

### Requirement: Scoring modes
A session SHALL support competitive, cooperative and team scoring.

#### Scenario: Competitive
- **WHEN** a session runs in competitive mode
- **THEN** each player has a score and the winner is the highest or lowest depending on the configured direction

#### Scenario: Cooperative
- **WHEN** a session runs in cooperative mode
- **THEN** the outcome is recorded as a table win or loss and no individual winner is set

#### Scenario: Teams
- **WHEN** a session runs in team mode
- **THEN** players are grouped into teams, the score is entered per team, and the winning team's players are recorded as winners

#### Scenario: Default mode
- **WHEN** a session is started for a game whose cooperative nature is known
- **THEN** the mode defaults accordingly and the user can override it

### Requirement: Table-side interface
The session interface SHALL be operable at arm's length with a thumb and without dense chrome.

#### Scenario: Target size and contrast
- **WHEN** the session page is rendered on a phone in landscape or a tablet in portrait
- **THEN** every interactive target is at least 48 pixels in its smallest dimension and text meets the design system's contrast requirement

#### Scenario: Keyboard operation
- **WHEN** a user operates the session page by keyboard
- **THEN** every score control, timer and action is reachable in a logical order with a visible focus indicator

#### Scenario: Score announcement
- **WHEN** a player's score changes
- **THEN** the new total is announced politely to assistive technology without announcing every keystroke

#### Scenario: No-JavaScript fallback
- **WHEN** the page is loaded without JavaScript
- **THEN** attendees and the game are still shown and the user is given a clear path to the standard play form
