## 1. Demo account

- [x] 1.1 Add `is_demo` to the user model and migration, and expose the demo configuration variables.
- [x] 1.2 Seed sample games and plays for the demo user at boot, and recreate the user if it is missing.
- [x] 1.3 Add `POST /auth/demo` to start a session for the demo user, gated by `DEMO_USER_ENABLED`.
- [x] 1.4 Show **Try the demo** on the login page when the demo is enabled.
- [x] 1.5 Add `GET /api/v1/auth/config` with the demo email and token and a **Try the demo account** option in mobile onboarding.

## 2. Privacy controls

- [x] 2.1 Add the per-user `analytics_opt_out` setting and `POST /settings/privacy` with CSRF protection.
- [x] 2.2 Skip the Umami script when the signed-in user has opted out.
- [x] 2.3 Add the opt-out control to Settings → Privacy controls.
- [x] 2.4 Delete setup-guide photo files from the setup media directory during account deletion.

## 3. Published legal text

- [x] 3.1 Update `privacy.html` for the analytics basis and opt-out, the objection right, setup-photo retention and the conditional demo section.
- [x] 3.2 Update `account_delete.html` to list setup photos and the demo recreation exception.
- [x] 3.3 Update `terms.html` with the shared-demo clause.
- [x] 3.4 Update `docs/legal.md` with demo, opt-out, media erasure and checklist sections.
- [x] 3.5 Update `docs/play-store.md` to use the demo flow for review.
- [x] 3.6 Document the demo variables in `.env.example` and `README.md`.

## 4. Verification

- [x] 4.1 Confirm the demo can sign in from the web and mobile onboarding, and that no admin route is reachable with the demo account.
- [x] 4.2 Confirm an opted-out account receives no Umami script and that clearing the flag restores it.
- [x] 4.3 Confirm account deletion removes setup-photo rows and files.
- [x] 4.4 Confirm the legal pages render signed out and name the demo only when it is enabled.
- [x] 4.5 Validate the OpenSpec change with `openspec validate --strict`.
