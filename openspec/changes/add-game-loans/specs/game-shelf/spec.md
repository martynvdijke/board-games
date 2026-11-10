## ADDED Requirements

### Requirement: Loan state on the shelf
The shelf SHALL show whether a game is currently lent.

#### Scenario: Grid and list
- **WHEN** a game has an open loan
- **THEN** the grid card shows a lent indicator and the list row shows the borrower's name

#### Scenario: Lend from the shelf
- **WHEN** a user activates the lend action on a game they own
- **THEN** the loan form opens without leaving the page context, and on save the shelf reflects the change without a full reload of unrelated data

#### Scenario: Not owned
- **WHEN** a game is not owned by the user
- **THEN** no lend action is offered
