# Legal, privacy and account deletion

This note explains how the published legal pages are built, what the account
deletion actually removes, and which operator values must be set before a store
submission.

## Published pages

Three routes are public and readable without a session:

| Route | Purpose |
| --- | --- |
| `GET /privacy` | GDPR Article 13 privacy notice |
| `GET /terms` | Acceptable-use terms |
| `GET /account/delete` | Deletion instructions and retention detail |

They use the normal `base.html` layout, but the signed-in navigation and footer
are hidden because there is no user. The text is rendered from configuration and
live deployment facts, not a static file, so the policy cannot claim a processor
the instance does not run.

## Operator configuration

Set these in the environment (or `.env`) and restart:

| Variable | Fallback | Meaning |
| --- | --- | --- |
| `LEGAL_CONTROLLER_NAME` | "The operator of this <app> instance" | Legal controller named on the policy |
| `LEGAL_CONTACT_EMAIL` | empty | Privacy and deletion contact; when empty the pages point at the signed-in route instead |
| `LEGAL_CONTACT_ADDRESS` | empty | Optional postal address |
| `LEGAL_HOSTING_REGION` | `the European Union` | Hosting / storage region |

The page also reads `BASE_URL`, `OIDC_ISSUER`, `UMAMI_URL` and
`OTEL_EXPORTER_OTLP_ENDPOINT` to decide whether to describe OIDC, analytics and
traces. BoardGameGeek and the hosting provider are always listed because the
application always uses them.

## What deletion does

`DELETE /api/v1/me` (personal token or browser session with CSRF) and
`POST /settings/account/delete` (CSRF-protected form with typed `DELETE`) call
the same service in `internal/account`.

One transaction:

1. Deletes browser sessions and personal API tokens.
2. Deletes `user_settings`.
3. Deletes `user_games` (shelf notes, location, condition, price, rating).
4. Deletes `play_players` rows belonging to the user's plays, then the plays.
5. Deletes `sync_runs`.
6. Deletes the `users` row.

Plays in this schema are owned by exactly one user and are not shared with other
accounts, so there is nothing to preserve by detaching authorship. If this
changes, add a nullable `created_by` marker instead of deleting the play and
update the public deletion page.

The last remaining admin cannot be deleted. The guard is part of the final
`DELETE` statement, so two admins deleting each other at the same time cannot
leave the instance without an administrator.

The browser session cookie is cleared and the API token stops working
immediately. The OIDC/Authelia identity is **not** deleted; that is a separate
account and the pages say so.

## Export

`GET /api/v1/me/export` returns JSON containing:

- profile (id, email, name, avatar URL, role, timestamps, last login)
- settings (BGG username, sync interval, theme, timezone)
- shelf entries (including private notes, location, condition and price)
- plays with their player names, scores and markers
- API token metadata (id, name, prefix, scopes, timestamps) — never hashes

The response is sent as an attachment and with `Cache-Control: no-store`.

## Third-party player names

`play_players.name` is free text and can identify someone who never used the
instance. Two routes handle that:

- `POST /settings/players/remove` (signed-in form, CSRF) with `name` and
  `mode=remove|anonymise`.
- `DELETE /api/v1/players?name=...&mode=remove|anonymise` (write scope).

A normal user is scoped to plays they own, so the route cannot read or change
another account's log. An admin may apply the change across the instance. The
public deletion page explains the email fallback for non-users.

## Backups

Deletion cannot reach backups that were taken before the request. Document the
backup rotation period and state clearly that backups are not used to restore an
individual account. See [backup.md](backup.md).
