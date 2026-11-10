## ADDED Requirements

### Requirement: Group-visible data is the only cross-user exception
User isolation SHALL remain absolute except for shelves a user has explicitly made visible to a group.

#### Scenario: Private data stays private
- **WHEN** a signed-in user requests a resource belonging to another user who has not shared a shelf with a group they both belong to
- **THEN** the response is not found

#### Scenario: Shared shelf is readable
- **WHEN** a user who has opted in and a viewer share an active group membership
- **THEN** the viewer may read that user's shelf contents as exposed by the group shelf rules and nothing else

#### Scenario: No configurable bypass
- **WHEN** any configuration or request parameter attempts to widen cross-user access beyond explicit group visibility
- **THEN** no such parameter is honoured

#### Scenario: Sessions unaffected
- **WHEN** group visibility is in use
- **THEN** session handling, CSRF protection and API token scoping are unchanged
