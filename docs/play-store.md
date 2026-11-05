# Google Play submission notes

This is the operator checklist for publishing the Flutter companion in
`mobile/`. Update it whenever the manifest, data model or API changes.

## Store listing

- App name: **The Shelf**
- Package: `xyz.vandijke.shelf`
- Privacy policy URL: `https://board-games.vandijke.xyz/privacy`
- Account deletion URL: `https://board-games.vandijke.xyz/account/delete`
- Support / privacy contact: set `LEGAL_CONTACT_EMAIL` and use the same address
  in the listing.

Google Play requires a deletion URL for any app that creates accounts. The
published page must stay reachable signed out; the handler tests assert that.

## Account deletion

Users can delete in two ways:

1. In the app: **Settings → Account → Delete account**, type `DELETE`, confirm.
   The app calls `DELETE /api/v1/me`, then clears the token and server URL from
   Android secure storage.
2. On the web: **Settings → Account**, type `DELETE`, submit. The same service
   runs.

If the user cannot sign in, the public deletion page gives the operator email
fallback. Deletion removes the account, settings, shelf, plays, player names,
sessions and API tokens. It does not delete the separate Authelia/OIDC identity,
and the app dialog warns about that before submission. Backups may retain old
copies until they rotate.

## App access for review

The app needs a working instance and a personal API token; OIDC happens on the
web only. The public host runs a shared demo account so reviewers do not need a
private login:

- Server URL: `https://board-games.vandijke.xyz`
- On the login page, choose **Try the demo**, or open
  `GET /api/v1/auth/config` and use the demo email and token returned there. In
  the mobile onboarding screen, choose **Try the demo account**.
- Demo email: the configured `DEMO_USER_EMAIL` (default `demo@theshelf.local`);
  display name: the configured `DEMO_USER_NAME` (default `Shelf Demo`).
- The account is public, non-admin and writable by anyone. The operator may reset
  it on restart, and sample data is recreated if the demo is deleted.

If the demo is disabled, or the reviewer needs a stable private account, fall
back to a dedicated review account in Authelia. Create it in advance and issue a
`read` + `write` personal token under **Settings → API tokens**. Do not publish
production admin credentials. Rotate or delete the private review token after
the review; the demo token can be rotated by changing `DEMO_USER_TOKEN`.

## Data safety form

Answer from the code, not aspiration. The current mapping:

| Data type | Collected | Shared | Purpose | Optional? | Notes |
| --- | --- | --- | --- | --- | --- |
| Email address | Yes | No | Account management, authentication | No | Comes from OIDC; stored in `users.email` |
| Name | Yes | No | Account management, display | No | OIDC display name |
| Profile photo / avatar URL | Yes | No | Display | Yes | OIDC avatar URL; empty if provider has none |
| User IDs | Yes | No | Account management | No | Internal user id |
| Other user-generated content | Yes | No | App functionality | Yes | Shelf notes, location, condition, play notes and free-text player names |
| App activity / in-app actions | Yes | No | App functionality | No | Plays, ratings, shelf state and sync history |
| App info and performance / diagnostics | Yes if OTLP is configured | No | Diagnostics | Yes | Server and client OpenTelemetry spans; disabled when endpoint is empty |
| Device or other IDs | No | No | — | — | No advertising ID, no push token |
| Location | No | No | — | — | The app has INTERNET only; shelf/play locations are typed text, not device location |
| Contacts / calendar / SMS / photos | No | No | — | — | Not requested |

Security practices:

- Data is encrypted in transit (HTTPS only; release builds do not allow
  cleartext).
- Users can request deletion from inside the app and from the public page.
- No data is sold; there are no ads or advertising SDKs.
- `android:allowBackup="false"`, `fullBackupContent="false"` and
  `data_extraction_rules.xml` exclude every domain from cloud backup and device
  transfer, because the bearer token lives in secure storage.

If Umami is enabled, add "App activity — page views" as collected for analytics,
not shared, with a privacy-policy link. If OTLP is unset, do not declare
diagnostics.

## Content rating and target audience

- Category: tools / entertainment reference, not a game with gambling or
  user-generated social feeds.
- Content: board-game collection metadata from BoardGameGeek and the user's own
  play logs. No violence, sexual content, profanity, gambling, drugs, or
  user-to-user messaging.
- Ads: **No ads**.
- Target audience: 16+ (the policy and terms exclude under-16 users). Do not
  opt into Designed for Families.
- News app: no. COVID/health: no. Financial features: no.

## Technical requirements

- `compileSdk` / `targetSdk` follow the Flutter SDK; the current Flutter stable
  targets **API 36**. Submit an Android App Bundle (`.aab`) targeting API 36 or
  newer, as Play requires.
- Permissions: `INTERNET` only. No camera, location, contacts or notification
  permission is requested.
- Cleartext traffic is disabled in the release manifest; debug builds may use
  `adb reverse` for local development.
- `MainActivity` is the only exported component and is the launcher activity.
- Signing: supply `mobile/android/key.properties` and a keystore outside the
  repository; CI also accepts the documented signing secrets.

## Before each release

1. Set `LEGAL_*` values and verify `/privacy`, `/terms` and `/account/delete`
   load signed out at the public URL.
2. Confirm **Try the demo** signs in on the web and **Try the demo account**
   works in a clean app install, and that both still point at a reachable
   server.
3. Run `go test ./...` and the mobile tests.
4. Create a fresh review token and confirm deletion works end to end with a
   throwaway account, including any uploaded setup photos.
5. Update the Data safety answers if a processor or permission changed.
