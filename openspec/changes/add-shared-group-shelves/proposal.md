## Why

The app assumes one person, one shelf: every query filters by `user_id` and
cross-user reads are 404 by design. Real game groups do not work that way — four
people own overlapping collections, and "what can we play tonight" is a question
about the union of four shelves, not one. This is the highest-value and highest-
risk item in the programme because it is the only one that relaxes an isolation
rule the codebase currently guarantees.

## What Changes

- Introduce **groups** (a household or regular play group) with explicit,
  invitation-based membership and an owner role. Membership is never inferred.
- Add a per-user **shelf visibility** setting: private (today's behaviour, the
  default), or visible to the groups the user belongs to. Nothing becomes
  visible without the user choosing it.
- Add a **group shelf** view: the union of members' visible collections, each
  game labelled with its owner, filterable by owner and by all existing facets.
- Add a **group planner mode** so a night plan can draw from the union, honouring
  each member's own visibility choice.
- Allow a play to name which copy was played, when several members own the same
  game, without merging their collection records.
- Preserve the isolation rule everywhere else: any resource not explicitly
  group-visible continues to answer 404, never 403. Group roles are enforced
  server-side on every group-scoped read and write.
- Add an audit trail for visibility changes and group membership changes, and
  make leaving a group immediately revoke access to that member's shelf.

### Non-goals

- Cross-instance federation or discovery of other instances' users.
- Shared editing of another member's shelf fields; visibility is read-only.
- Group-level billing, quotas, or administration beyond owner and member.
- Public or anonymous group pages.
- Changing how BGG sync assigns ownership; each member syncs their own.

## Capabilities

### New Capabilities

- `shared-shelves`: Groups, invitations, membership roles, per-user shelf
  visibility, the group shelf union view, and group-scoped planning.

### Modified Capabilities

- `oidc-authentication`: The user-isolation requirement gains one explicit,
  opt-in exception for group-visible data, and the 404 rule is restated for
  everything else.
- `game-shelf`: The shelf gains a group scope and an owner facet.

## Impact

- New migration: `groups`, `group_members`, `group_invites`, `shelf_visibility`
  on user settings, and an audit table.
- `internal/collection/repo.go`: every query gains an explicit scope parameter;
  this is the riskiest change in the programme and touches the whole read path.
- `internal/auth`: group and role authorisation helpers used by handlers.
- `internal/web/templates`: group settings, invitation acceptance, group shelf.
- `internal/game-night-planner`: accepts a group scope.
- `internal/api`: group resources; all existing endpoints unchanged.
- `docs/api.md`, OpenAPI document, `README.md`, privacy page, `.env.example`.
- Test strategy: isolation tests must be extended, not merely kept passing;
  every group-visible path needs a negative test proving 404 for non-members.
