## ADDED Requirements

### Requirement: Loan lifecycle
The system SHALL record that a game has been lent, to whom, since when, and when it was returned, allowing at most one open loan per game.

#### Scenario: Lending a game
- **WHEN** a user marks an owned game as lent and supplies a borrower name
- **THEN** an open loan is created with the lender's recorded date, and the game's detail page and list card show that it is out

#### Scenario: Second open loan is refused
- **WHEN** a user tries to lend a game that already has an open loan
- **THEN** the action is refused with an explanation and the existing loan is unchanged

#### Scenario: Returning a game
- **WHEN** a user marks an open loan as returned
- **THEN** the loan is closed with a returned date, remains in history, and the game shows as available again

#### Scenario: Optional due date
- **WHEN** a loan is created without a due date
- **THEN** it is tracked as an idle loan with no overdue state

### Requirement: Overdue visibility
Overdue loans SHALL be visible without being stored as a status.

#### Scenario: Loans page ordering
- **WHEN** a user opens the loans page
- **THEN** open loans are listed oldest lending date first, overdue loans are marked, and closed loans are available separately

#### Scenario: Due date passes
- **WHEN** the current date passes a loan's due date
- **THEN** the loan is shown as overdue on the next render without any migration or scheduled job

### Requirement: Loan reminders
The system SHALL send loan reminders only when reminders are enabled and a notifier is configured.

#### Scenario: Reminders disabled
- **WHEN** reminders are not enabled in configuration
- **THEN** no loan notification is sent and the loans page still identifies overdue loans

#### Scenario: Overdue reminder sent once
- **WHEN** the scheduler finds an overdue open loan that has not been notified within the configured interval
- **THEN** one notification is sent and the loan records when it was last notified

#### Scenario: No duplicate on restart
- **WHEN** the server restarts immediately after sending a reminder
- **THEN** the same loan is not reminded again until the interval elapses, because the notification time is stored

### Requirement: Ownership changes close loans
An open loan SHALL NOT outlive the user's ownership of the game.

#### Scenario: Game leaves the collection
- **WHEN** a sync removes a lent game from the user's collection
- **THEN** its open loan is closed with a note explaining the ownership change and no further reminders are sent

#### Scenario: Game is deleted
- **WHEN** a lent game's record is deleted
- **THEN** its loans are removed with it rather than left orphaned
