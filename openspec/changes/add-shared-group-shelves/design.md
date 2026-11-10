## Context

This is the most invasive change in the programme. Today the isolation rule is
absolute: every query filters by `user_id`, and `AGENTS.md` states that cross-user
reads must answer 404, not 403. Sessions are database-backed, CSRF is enforced on
state-changing browser requests, and API requests authenticate by bearer token and
bypass CSRF.

Constraints: SQLite single writer, no nesting transactions, and an explicit
requirement not to weaken isolation by accident.

## Goals / Non-Goals

**Goals:**
- Let a game group see the union of shelves that members opted to share.
- Keep the default identical to today: nothing is visible unless chosen.
- Make every new read path provably non-leaking, with tests that fail loudly.

**Non-Goals:**
- Shared editing of another member's shelf fields.
- Public or anonymous group pages.
- Federation or discovery of users on other instances.

## Decisions

**Visibility is a separate, explicit per-user setting, default `private`.**
Storing `shelf_visibility ∈ {private, groups}` on user settings means the default
deploy changes nothing observable. A user opting in shares with *the groups they
belong to*, not with the instance.

**Scope is an explicit parameter threaded through the read path, not a global
switch.** `collection.Scope{Kind: User|Group, UserID, GroupID}` is required by
group-capable queries, so adding a new query without choosing a scope is a compile
error rather than a silent leak. *Alternative rejected:* middleware that rewrites
`user_id` filters, which is invisible at the call site and the exact class of bug
that leaks data.

**Authorisation is a single helper, `authz.CanViewShelf(ctx, viewer, targetUser)`,
used by every group-visible read.** It returns true only when the target's
visibility is `groups`, the viewer and target share at least one group, and the
target's membership is active. Group roles are checked by a sibling helper for
writes (invitations, membership, visibility).

**The group shelf is a union with owner labels, never a merge.** Two members
owning the same BGG game produce two rows labelled with each owner, because their
personal fields differ. Deduplication is a display concern: a grouped view can
show one card with "owned by 2" and expand to per-owner detail, but the underlying
records stay separate.

**A play names the copy it was played with.** `plays` gains a nullable
`copy_owner_id` when the session originated from a group shelf. This preserves
accurate per-owner statistics without moving plays between accounts.

**Leaving a group revokes immediately and auditably.** Membership rows are
soft-deleted with a timestamp; every authorisation check requires an active
membership, so revocation needs no cache invalidation. An audit row records who
changed visibility, who invited whom, and when.

**Group planning reuses the planner with a scope.** `game-night-planner` already
takes a candidate query; a group scope passes a different query and the same pure
ranking function. No new ranking logic.

**API: group resources are additive; existing endpoints never change shape.** A
group-visible game reached through an existing endpoint without the scope
parameter still 404s, preserving current semantics for every current client.

## Risks / Trade-offs

- **The isolation guarantee is weakened** → default private, explicit scope
  parameter, a dedicated negative-test suite for every group path, and a
  documented threat model in the design. This is the primary risk of the whole
  programme and the reason it ships last.
- **A member shares more than they intended** → the visibility setting names
  exactly which groups can see the shelf, and shows a preview of what a group
  member would see.
- **Ownership ambiguity for duplicate games** → owner labels everywhere in group
  context; no silent merging of collection records.
- **Index and query cost for union reads** → union queries are bounded by group
  size; add indexes on membership lookups and verify pagination still uses the
  owner-scoped path.
- **Test suite previously assumed 404 everywhere** → extend, do not relax: every
  existing isolation test stays, and new tests assert the single intended
  exception.

## Migration Plan

1. Add `groups`, `group_members`, `group_invites`, `shelf_visibility`, audit table.
   All additive; nothing reads them yet.
2. Ship authorisation helpers and the 404-preserving behaviour with no UI.
3. Ship group settings and invitations.
4. Ship the group shelf view and group planning.
Rollback at any step: earlier steps are inert without UI, and the default
visibility keeps the old behaviour.

## Open Questions

- Should group membership be limited to a maximum size? A cap keeps union queries
  and the planner honest, but the number is arbitrary; leaning toward no cap but
  a paginated group shelf.
- Do groups need a name uniqueness constraint per instance? Likely yes, to avoid
  two identically named groups in an invitation dialog.
- Is an invitation code enough, or do we need an email path? Codes only for now,
  consistent with the instance's opt-out-of-email posture.
