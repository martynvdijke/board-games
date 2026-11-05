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
application always uses them. When `DEMO_USER_ENABLED` is on, the legal pages
also name the demo account.

## Demo account

Some instances publish a shared demo so people can try the app without an OIDC
account. It is a normal non-admin user with `is_demo` set, created from
`DEMO_USER_EMAIL` (default `demo@theshelf.local`) and `DEMO_USER_NAME` (default
`Shelf Demo`). It is enabled by `DEMO_USER_ENABLED`, which defaults to on only
for the host `board-games.vandijke.xyz`, and it uses the personal API token in
`DEMO_USER_TOKEN`.

Purpose: let reviewers and visitors open a working shelf with sample games and
plays, from the web login page and from the mobile onboarding screen.

Exposure: the account is public. Anyone can sign in to it through
`POST /auth/demo`, read it, and write to it; the same token is published by
`GET /api/v1/auth/config` for the mobile app. It is not an administrator and
cannot reach other users' data.

Reset: the operator may reset the demo at any time, including on restart. If the
demo user is missing at server boot it is recreated with sample data. Anything
a visitor changed may therefore disappear.

No real personal data belongs in it. The sample content is seeded, not taken
from a real account. The privacy page tells visitors not to enter personal
data, and the terms forbid personal or unlawful content there. Deleting the demo
account is allowed, but the account is recreated at the next restart if it is
missing; demo data is not personal data and is not treated as an erasure
request.

## Analytics opt-out

Umami is optional and cookieless. A signed-in user can object to analytics
without contacting the operator: **Settings → Privacy controls** submits
`POST /settings/privacy` and stores a per-user `analytics_opt_out` flag on the
user's settings row. While the flag is set, the Umami script is not rendered for
that user, so no page views from their sessions reach Umami. The choice belongs
to the account and is deleted with it. Turning the setting back on removes the
objection; nothing else about the account changes.

This is the GDPR Article 21 objection route for analytics. The legal basis for
analytics remains legitimate interest, so the objection is honoured without
needing a consent banner.

## Setup-media erasure

Uploaded setup-guide photos are stored as files in the setup media directory
(`setup/` next to the SQLite database by default) with a row per image in the
database. Removing a photo deletes both. Account deletion deletes every image
row belonging to the user inside the main transaction, then removes the
matching files on disk after that transaction commits. A missing file is
ignored, and a file that cannot be removed is logged but does not undo the
account deletion.

## What deletion does

`DELETE /api/v1/me` (personal token or browser session with CSRF) and
`POST /settings/account/delete` (CSRF-protected form with typed `DELETE`) call
the same service in `internal/account`.

One transaction:

1. Deletes browser sessions and personal API tokens.
2. Deletes `user_settings`, including the analytics opt-out flag.
3. Deletes `user_games` (shelf notes, location, condition, price, rating).
4. Deletes `play_players` rows belonging to the user's plays, then the plays.
5. Deletes `game_setup_images` rows.
6. Deletes `sync_runs`.
7. Deletes the `users` row.

After the commit, the service removes the corresponding files from the setup
media directory. File removal is not part of the transaction: a missing file is
ignored, and a file that cannot be removed is logged. If the transaction rolls
back, no files are removed.

The public demo account follows the same path, but the boot-time demo seed
recreates it at the next start if it is missing. That recreated data is sample
data, not the personal data of the person who deleted the demo.

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

## EU launch checklist

- [ ] Set `LEGAL_CONTROLLER_NAME`, `LEGAL_CONTACT_EMAIL`, `LEGAL_CONTACT_ADDRESS`
      and `LEGAL_HOSTING_REGION` for the instance.
- [ ] Confirm `/privacy`, `/terms` and `/account/delete` load signed out.
- [ ] Confirm the privacy page describes OIDC, analytics and traces only when
      those features are configured.
- [ ] Exercise account deletion with a throwaway account and check the shelf,
      plays, player names, tokens, sessions and setup photos are gone.
- [ ] State the real backup rotation period; do not promise a fixed number here
      until the operator's schedule is known.

## GDPR checklist

- [ ] The privacy page names the controller, contact route, hosting region and
      the legal basis for each processing purpose.
- [ ] Analytics is described with legitimate interest, and the page explains the
      per-user opt-out under Settings → Privacy controls.
- [ ] `POST /settings/privacy` stores `analytics_opt_out` and the Umami script
      is not rendered when it is set.
- [ ] The demo account is disclosed as public, writable by anyone, non-admin,
      recreated at boot if missing and unsuitable for personal data.
- [ ] Deleting an account removes uploaded setup-guide photos and files, not
      only the database rows.
- [ ] The deletion page explains the demo exception and the separate OIDC
      identity.
