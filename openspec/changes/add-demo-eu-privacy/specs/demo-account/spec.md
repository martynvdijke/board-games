## ADDED Requirements

### Requirement: Public demo account
The server SHALL provide a shared, non-admin demo user, seeded with sample games
and plays, when the demo is enabled.

#### Scenario: Demo is enabled
- **WHEN** `DEMO_USER_ENABLED` is true
- **THEN** a user with `is_demo` set exists using `DEMO_USER_EMAIL` and `DEMO_USER_NAME`, and sample games and plays are available on its shelf

#### Scenario: Demo user is missing at boot
- **WHEN** the server starts and the demo is enabled but the demo user does not exist
- **THEN** the user and its sample data are created

#### Scenario: Demo is disabled
- **WHEN** `DEMO_USER_ENABLED` is false
- **THEN** no demo user is created, `POST /auth/demo` does not start a session, and the login page does not offer the demo

#### Scenario: Demo account is deleted
- **WHEN** an account deletion removes the demo user while the server is running
- **THEN** the demo is unavailable until the next restart, when it is recreated with sample data

### Requirement: Web demo sign-in
The web app SHALL let a visitor start a session for the demo user without an OIDC round trip while the demo is enabled.

#### Scenario: Visitor chooses the demo
- **WHEN** a visitor submits `POST /auth/demo`
- **THEN** a browser session is created for the demo user and the visitor lands in the app as that user

### Requirement: Mobile demo credentials
The public API SHALL expose the demo email and token so the mobile app can offer a demo sign-in without a private review account.

#### Scenario: Mobile onboarding requests auth config
- **WHEN** a client sends `GET /api/v1/auth/config`
- **THEN** the response includes the demo email and the published demo token when the demo is enabled

### Requirement: Demo safety limits
The demo SHALL remain non-admin and SHALL NOT contain real personal data.

#### Scenario: Demo session is used
- **WHEN** a visitor is signed in as the demo user
- **THEN** administrative routes remain unavailable and the data visible is the seeded sample data plus changes made by other visitors

#### Scenario: Demo token is configured
- **WHEN** the demo is enabled in production
- **THEN** `DEMO_USER_TOKEN` is required and starts with `bg_`
