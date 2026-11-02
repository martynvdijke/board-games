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
web only.

Provide reviewers with:

- Server URL: `https://board-games.vandijke.xyz`
- A dedicated review account in Authelia, created in advance.
- A `read` + `write` personal token for that account, created under
  **Settings → API tokens** and pasted into the app's onboarding screen.
- Instructions: enter the server URL and token, then use Shelf, Play log,
  Stats and Settings. Do not publish production admin credentials.

Rotate or delete the review token after the review.

## Data safety form

Answer from the code, not aspiration. The current mapping:

| Data type | Collected | Shared | Purpose | Optional? | Notes |
| --- | --- | --- | --- | --- | --- |
| Email address | Yes | No | Account management, authentication | No | Comes from OIDC; stored in `users.email` |
| Name | Yes | No | Account management, display | No | OIDC display name |
| Profile photo / avatar URL | Yes | No | Display | Yes | OIDC avatar URL; empty if provider has none |
| User IDs | Yes | No | Account management | No | Internal user id and OIDC subject |
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
   load signed out.
2. Run `go test ./...` and the mobile tests.
3. Create a fresh review token and confirm deletion works end to end with a
   throwaway account.
4. Update the Data safety answers if a processor or permission changed.
