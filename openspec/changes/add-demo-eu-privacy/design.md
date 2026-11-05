## Context

The app authenticates through OIDC, so a store reviewer needs either a real
identity or a token issued from one. The mobile app already accepts a server URL
and a personal API token during onboarding. The public host is
`board-games.vandijke.xyz`.

The privacy pages are rendered from live configuration, so features that are not
running are not described. Umami is optional and cookieless but is still a
processing activity under legitimate interest, so a user needs an objection
route that takes effect without operator involvement.

Uploaded setup-guide photos live as files on disk plus rows in the database.
Deleting only the rows leaves the images behind.

## Goals / Non-Goals

**Goals:**
- Give Play reviewers a working account without a manual private credential.
- Keep the demo clearly public, non-admin and disposable.
- Honour an analytics objection at the point the script would be rendered.
- Remove setup-photo files when an account is deleted.
- Keep the published legal text accurate for enabled features.

**Non-Goals:**
- Per-visitor demo sandboxes.
- Demo write protection.
- A cookie-consent banner or a second analytics provider.
- Changing backup rotation or promising a retention number.

## Decisions

1. **One shared demo user, not per-session users** — the demo is a normal user
   row with `is_demo` set. This keeps login, shelf and API paths unchanged and
   lets the mobile app use an ordinary personal token. The trade-off is that
   visitors can change each other's demo content; that is acceptable because the
   data is seeded and may be reset, and the account is recreated if missing and
   is non-admin.
2. **Explicit enablement with a host default** — `DEMO_USER_ENABLED` controls
   the feature. It defaults on only for `board-games.vandijke.xyz`, so private
   instances and local development do not publish a writable account by
   accident.
3. **Token supplied from the environment** — `DEMO_USER_TOKEN` must start with
   `bg_` and is published by the API. This is deliberate for a demo; the token
   has read and write scope but no admin rights. Rotating it is a config change
   and restart.
4. **Recreate at boot** — if the demo user is missing when the server starts, it
   is created and the sample games and plays are seeded. If it exists, it is
   left in place. This makes deletion recoverable but means visitor changes are
   temporary.
5. **Opt out by not rendering the script** — `POST /settings/privacy` stores
   `analytics_opt_out` on the user's settings row. Page rendering checks the
   flag and omits the Umami script entirely. No consent cookie is introduced.
   The flag is deleted with the account.
6. **Delete media files during account deletion** — the existing transaction
   deletes image rows, and the service removes the corresponding files in the
   setup media directory only after that transaction commits. A missing file is
   ignored. A failed transaction leaves both rows and files intact. A crash
   between the commit and file removal can leave orphaned files on disk, and a
   repeat deletion cannot find the rows to retry them.

## Risks / Trade-offs

- **[Public write access]** -> Anyone can deface the demo. It is seeded,
  non-admin, recreated if missing and carries no real data; the terms forbid
  personal or unlawful content.
- **[Token exposure]** -> `DEMO_USER_TOKEN` is public by design. It must not be
  reused for another account and must be rotated by changing the environment
  variable.
- **[Demo deletion is not permanent]** -> Deleting the demo user does not erase
  sample data permanently because boot recreates it if missing. The legal pages
  say so and describe demo data as non-personal.
- **[File deletion is outside the transaction]** -> Files are removed only after
  a successful commit, so a failed transaction leaves both rows and files
  intact. A crash between the commit and file removal can leave orphaned files
  on disk, and a repeat deletion cannot find the rows to retry them.
- **[Analytics signal is reduced]** -> Opted-out users are invisible to Umami,
  so analytics covers only users who have not objected. That is the intended
  effect of an Article 21 objection.
