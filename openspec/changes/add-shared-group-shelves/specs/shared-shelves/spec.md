## ADDED Requirements

### Requirement: Groups and membership
The system SHALL support groups with explicit, invitation-based membership and an owner role.

#### Scenario: Creating a group
- **WHEN** a signed-in user creates a group
- **THEN** they become its owner and the group has no other members

#### Scenario: Invitation
- **WHEN** an owner creates an invitation
- **THEN** a single-use code is produced and the group gains no member until the code is accepted

#### Scenario: Accepting an invitation
- **WHEN** a signed-in user accepts a valid invitation code
- **THEN** they become an active member of that group and the code can no longer be used

#### Scenario: Leaving a group
- **WHEN** a member leaves a group
- **THEN** their membership becomes inactive immediately and the group no longer sees their shelf

#### Scenario: Owner departure
- **WHEN** the last owner leaves a group with other members
- **THEN** ownership transfers to another member rather than leaving the group ownerless

### Requirement: Shelf visibility
Each user SHALL control whether their shelf is visible to the groups they belong to, and private SHALL be the default.

#### Scenario: Default is private
- **WHEN** a user has never changed the setting
- **THEN** no other user can see any part of their shelf

#### Scenario: Opting in
- **WHEN** a user sets their shelf visible to groups
- **THEN** members of groups they belong to can view their shelf, and the user is shown which groups that means

#### Scenario: Opting out
- **WHEN** a user returns the setting to private
- **THEN** group members lose access immediately, with no cached or delayed visibility

#### Scenario: Preview
- **WHEN** a user is choosing their visibility
- **THEN** they can preview what a group member would see before confirming

### Requirement: Group shelf
The system SHALL present the union of members' visible collections with owner labels.

#### Scenario: Union view
- **WHEN** a member opens a group shelf
- **THEN** it shows games from all members whose visibility permits it, each labelled with its owner

#### Scenario: Duplicate games
- **WHEN** two members own the same game
- **THEN** both records are represented and the view can group them while preserving per-owner detail

#### Scenario: Owner filtering
- **WHEN** a member filters the group shelf by owner
- **THEN** only that owner's visible games are shown

#### Scenario: Nothing shared
- **WHEN** no member has opted in
- **THEN** the group shelf explains that no shelves are shared rather than appearing broken

### Requirement: Group-scoped planning
A night plan SHALL be evaluable against a group's union shelf.

#### Scenario: Planning from a group
- **WHEN** a member plans a night with a group scope
- **THEN** candidates come from the union of opted-in members' collections and each result names the owner of the game

#### Scenario: Respecting visibility
- **WHEN** a member opts out of sharing
- **THEN** their games immediately stop appearing as group planning candidates

### Requirement: Authorisation and audit
Group access SHALL be authorised server-side on every group-scoped request, and visibility and membership changes SHALL be recorded.

#### Scenario: Non-member access
- **WHEN** a user requests a group resource for a group they do not belong to
- **THEN** the response is not found

#### Scenario: Non-shared shelf access
- **WHEN** a user requests another user's shelf while that user's visibility is private
- **THEN** the response is not found, not forbidden

#### Scenario: Role enforcement
- **WHEN** a non-owner attempts to create an invitation or change membership
- **THEN** the action is refused

#### Scenario: Audit trail
- **WHEN** a user changes shelf visibility or a group's membership changes
- **THEN** an audit entry records who, what and when
