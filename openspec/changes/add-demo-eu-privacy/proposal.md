## Why

Google Play review needs a way into the app without an Authelia account. A
private review token has to be created by hand for each submission and rotated
afterwards, which is easy to forget. The public instance can instead run a
shared demo account that reviewers and visitors can use directly.

At the same time, the published legal pages need to describe the demo before it
is switched on, and users need a self-service way to object to analytics. Account
deletion also needs to remove uploaded setup-guide photos, not only the database
rows that point at them.

## What Changes

- Add a shared, non-admin user marked `is_demo`, created from sample games and
  plays at server boot when `DEMO_USER_ENABLED` is on. The account is public and
  writable by anyone, and is recreated if missing.
- Add `POST /auth/demo` and a **Try the demo** button on the login page while
  the demo is enabled.
- Add a public `GET /api/v1/auth/config` response with the demo email and token,
  and a **Try the demo account** option in mobile onboarding.
- Add a per-user `analytics_opt_out` flag, changed at `POST /settings/privacy`.
  When it is set, the Umami script is not loaded for that user.
- Delete uploaded setup-guide photo files from the setup media directory during
  account deletion, in addition to the existing database rows.
- Extend the privacy, terms and account-deletion pages, the operator legal note,
  the Play Store notes and the configuration reference for the demo and the
  analytics objection.
- Add the demo variables `DEMO_USER_ENABLED`, `DEMO_USER_EMAIL`,
  `DEMO_USER_NAME` and `DEMO_USER_TOKEN`.

### Non-goals

- Isolating demo data per visitor or giving each visitor a separate sandbox.
- Allowing the demo to reach other users' data or administer the instance.
- A general consent-management platform; the analytics objection is one
  per-user setting, not a cookie banner.
- Storing real personal data in the demo.
- Promising a fixed backup retention period.

## Capabilities

### New Capabilities

- `demo-account`: A shared public account for Play review and visitor trials,
  seeded with sample data and recreated at boot if missing.
- `privacy-rights`: Self-service analytics objection and truthful disclosure of
  the demo and setup-media deletion.

### Modified Capabilities

- None.

## Impact

- New demo user creation and boot seeding in the server startup path.
- New web route `POST /auth/demo` and a login-page control.
- New public API response `GET /api/v1/auth/config`.
- New settings route `POST /settings/privacy` and a per-user settings column.
- Account deletion now removes files in the setup media directory.
- Updated `internal/web/templates/privacy.html`, `terms.html` and
  `account_delete.html`.
- Updated `docs/legal.md`, `docs/play-store.md`, `.env.example` and `README.md`.
