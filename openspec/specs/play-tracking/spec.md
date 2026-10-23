# play-tracking Specification

## Purpose
TBD - created by archiving change add-board-games-v1. Update Purpose after archive.
## Requirements
### Requirement: Users SHALL log plays locally

An authenticated user SHALL be able to log a play for any game on their shelf with a play date, optional location, duration in minutes, quantity, notes, and zero or more players. Each player SHALL have a name, optional score, optional winner flag, and optional first-play flag.

#### Scenario: Log a simple play
- **WHEN** a user submits a play with a date and game
- **THEN** the play SHALL be stored and appear in the timeline
- **AND** the game's local play count SHALL increase

#### Scenario: Log a play with players and scores
- **WHEN** a user adds three players with scores and marks one winner
- **THEN** all players and scores SHALL be stored with the play
- **AND** the winner SHALL be identifiable in the timeline and statistics

#### Scenario: Validation
- **WHEN** a play is submitted with a missing game, an invalid date, a negative duration, or a duplicate player name
- **THEN** the request SHALL be rejected with a readable validation message
- **AND** no partial play SHALL be stored

#### Scenario: Future date
- **WHEN** a play date is in the future
- **THEN** the system SHALL reject it unless the date is today in the user's configured timezone

### Requirement: Users SHALL edit and delete plays

The owner of a play SHALL be able to update its date, location, duration, quantity, notes, and players, and to delete it. Edits SHALL be reflected in timelines and statistics immediately.

#### Scenario: Edit a play
- **WHEN** a user changes the score of a player
- **THEN** the stored score SHALL update
- **AND** the timeline SHALL show the new score

#### Scenario: Delete a play
- **WHEN** a user deletes a play after confirming
- **THEN** the play and its players SHALL be removed
- **AND** play counts SHALL decrease

#### Scenario: Delete another user's play
- **WHEN** a user attempts to edit or delete a play they do not own
- **THEN** the system SHALL return `404 Not Found`

### Requirement: Plays SHALL be presented as a timeline

The plays page SHALL list plays in reverse chronological order, grouped by month, with the game cover and title, date, players, scores, winner markers, duration, location, and notes. It SHALL support filtering by game and year.

#### Scenario: Timeline grouping
- **WHEN** a user opens the plays page with plays across multiple months
- **THEN** plays SHALL be grouped under month headings in reverse chronological order

#### Scenario: Filter by game
- **WHEN** a user filters plays for a specific game
- **THEN** only plays of that game SHALL be listed

#### Scenario: Empty timeline
- **WHEN** the user has no plays
- **THEN** the page SHALL show a friendly empty state with a log-play action

### Requirement: Imported and local plays SHALL be distinguishable

Plays imported from BoardGameGeek SHALL be marked with their origin and BGG play identifier. Locally logged plays SHALL be marked as local. The interface SHALL show the origin where it helps the user understand sync behaviour.

#### Scenario: Imported play label
- **WHEN** a play was imported from BGG
- **THEN** the timeline SHALL indicate its BGG origin

#### Scenario: Local play label
- **WHEN** a play was created in this application
- **THEN** the timeline SHALL indicate that it is local and not yet on BGG

### Requirement: The system SHALL provide collection and play statistics

The statistics page SHALL show plays per month for the current year, total plays, most-played games, a player leaderboard by wins and plays, the collection split by weight band, and the shelf-of-shame count. Statistics SHALL be computed from the authenticated user's data only.

#### Scenario: Plays per month
- **WHEN** a user has plays in several months
- **THEN** the statistics page SHALL show a monthly count for each month with data

#### Scenario: Player leaderboard
- **WHEN** plays include player names and winners
- **THEN** the leaderboard SHALL rank players by wins with plays as a secondary count

#### Scenario: Empty statistics
- **WHEN** the user has no plays
- **THEN** the statistics page SHALL show zero states without errors

