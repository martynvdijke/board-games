## 1. Data model

- [ ] 1.1 Add migrations for `groups`, `group_members`, `group_invites` and an audit table
- [ ] 1.2 Add `shelf_visibility` to user settings defaulting to private
- [ ] 1.3 Add indexes for membership lookups and group-scoped reads
- [ ] 1.4 Add nullable `plays.copy_owner_id` for plays attributed to another member's copy

## 2. Authorisation first

- [ ] 2.1 Introduce `collection.Scope` and require an explicit scope on every group-capable query
- [ ] 2.2 Implement `authz.CanViewShelf` and the role helper for group writes
- [ ] 2.3 Write the negative test suite first: every group path returns not found for non-members and for private shelves
- [ ] 2.4 Extend, never relax, the existing user-isolation tests

## 3. Groups and visibility

- [ ] 3.1 Implement group creation, invitation codes, acceptance, leaving and owner transfer
- [ ] 3.2 Build group settings and invitation acceptance pages with CSRF protection
- [ ] 3.3 Build the visibility setting with a preview of what a group member would see
- [ ] 3.4 Make leaving a group revoke access immediately with no cache to invalidate
- [ ] 3.5 Write audit entries for visibility and membership changes

## 4. Group shelf

- [ ] 4.1 Add the group shelf route with the union query and owner labels
- [ ] 4.2 Add the owner facet alongside existing filters and a scope switcher
- [ ] 4.3 Group duplicate games for display while preserving per-owner detail
- [ ] 4.4 Exclude private personal fields such as notes, price paid and setup guides from group views
- [ ] 4.5 Add the empty state for a group with no shared shelves

## 5. Group planning

- [ ] 5.1 Accept a group scope in the planner candidate query
- [ ] 5.2 Name the game's owner in group planner results
- [ ] 5.3 Ensure an opt-out removes a member's games from planning immediately

## 6. Documentation and verification

- [ ] 6.1 Update `docs/api.md`, the OpenAPI document, `README.md` and the privacy page to describe group visibility
- [ ] 6.2 Test that a non-member receives not found for every group resource and for a private shelf
- [ ] 6.3 Test that a non-owner cannot invite or change membership
- [ ] 6.4 Test that leaving a group removes the shelf from other members' views on the next request
- [ ] 6.5 Test that personal fields never appear in a group shelf response
- [ ] 6.6 Run the full isolation suite and confirm no existing negative test was weakened
