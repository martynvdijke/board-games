## ADDED Requirements

### Requirement: Loan API resources
The API SHALL expose loan resources scoped to the authenticated user.

#### Scenario: Listing loans
- **WHEN** an authenticated client lists loans
- **THEN** it receives the user's open and closed loans with game reference, borrower, dates, and derived overdue state

#### Scenario: Creating a loan
- **WHEN** an authenticated client creates a loan for a game the user owns that has no open loan
- **THEN** the loan is created and returned with its identifier

#### Scenario: Conflicting loan
- **WHEN** a client creates a second open loan for the same game
- **THEN** the API returns a deterministic conflict error in the standard error envelope

#### Scenario: Another user's loan
- **WHEN** a client requests a loan identifier that belongs to another user
- **THEN** the API responds with not found rather than forbidden
