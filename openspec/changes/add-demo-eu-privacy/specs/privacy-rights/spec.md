## ADDED Requirements

### Requirement: Analytics objection
A signed-in user SHALL be able to object to analytics from Settings, and the objection SHALL stop the analytics script from loading for that user.

#### Scenario: User opts out
- **WHEN** a signed-in user submits `POST /settings/privacy` to opt out
- **THEN** `analytics_opt_out` is stored on the user's settings and the Umami script is not rendered for that user

#### Scenario: User opts back in
- **WHEN** the same user clears the opt-out
- **THEN** the Umami script is rendered again according to the instance configuration

#### Scenario: Analytics is not configured
- **WHEN** `UMAMI_URL` is empty
- **THEN** no analytics script is rendered for any user and the privacy page does not claim analytics is running

### Requirement: Demo disclosure
The published privacy and terms pages SHALL describe the demo when it is enabled.

#### Scenario: Demo is enabled
- **WHEN** `/privacy` or `/terms` is rendered while the demo is enabled
- **THEN** the page states that the demo is shared and public, writable by anyone, non-admin, may be reset on restart, and unsuitable for personal data, and the privacy page names `DEMO_USER_EMAIL`

#### Scenario: Demo is disabled
- **WHEN** `/privacy` or `/terms` is rendered while the demo is disabled
- **THEN** the page does not claim a demo account exists

### Requirement: Setup-photo erasure
Account deletion SHALL remove uploaded setup-guide photos, both database rows and stored files.

#### Scenario: Account with setup photos is deleted
- **WHEN** a user with uploaded setup-guide photos deletes the account
- **THEN** the image rows are deleted with the account and the matching files are removed from the setup media directory

#### Scenario: A setup file is already missing
- **WHEN** deletion encounters a missing setup-photo file
- **THEN** deletion continues and the remaining account data is still removed

### Requirement: Retention disclosure
The privacy page SHALL state that uploaded setup-guide photos are deleted with the account.

#### Scenario: Visitor reads retention
- **WHEN** `/privacy` is rendered
- **THEN** the retention section says setup-guide photos are kept until removed or the account is deleted, and lists them among the data removed on deletion
