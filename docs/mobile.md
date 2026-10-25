# Android companion

The Android app is a Flutter client for the same REST API as the web UI. It
lives in `mobile/` and ships as a sideloadable APK. It is intentionally a
companion, not an offline copy: the server remains the source of truth.

## Install

Download `the-shelf-<version>-android.apk` from a GitHub release, or build it
yourself:

```sh
cd mobile
flutter pub get
flutter build apk --release
adb install build/app/outputs/flutter-apk/app-release.apk
```

Release builds require Java 17. On a machine where `java` points at an older
JDK:

```sh
JAVA_HOME=/usr/lib/jvm/java-17-openjdk PATH=/usr/lib/jvm/java-17-openjdk/bin:$PATH \
  flutter build apk --release
```

If `mobile/android/key.properties` is absent, release builds are signed with
the debug key so local installs still work. For a real signing key, copy
`mobile/android/key.properties.example` and point it at a keystore that is
never committed. CI uses the optional repository secrets
`ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`,
and `ANDROID_KEY_PASSWORD`; without them the release workflow produces a
debug-signed APK.

## Connect

1. Open **Settings → API tokens** on the web app.
2. Create a token with the `read` and `write` scopes.
3. In the Android app, enter your server URL (for example
   `https://shelf.example.com`) and paste the token.
4. The app validates the token against `GET /api/v1/me` and stores both values
   in Android secure storage.

Use a trusted HTTPS certificate. Cleartext HTTP is enabled only in debug
builds for local development. With `adb reverse tcp:6290 tcp:6290`, a debug
build can reach a local server at `http://127.0.0.1:6290`.

Signing out clears the server URL and token; theme preference is kept.

## What works

- Onboarding and token validation
- Shelf grid with stats, search, status filters, and sorting
- Game detail with BGG metadata, description, shelf location/condition/notes
- Shelf edits: favourite, rating, location, condition, notes
- Play timeline grouped by month, with local edit/delete
- Log a play with date, location, duration, rounds, players, scores, winners,
  and notes
- Yearly statistics, monthly chart, most played, leaderboard, weight bands,
  and shelf of shame
- Theme: system, light, or dark
- Trigger a BGG sync and watch its progress

BoardGameGeek-origin plays are read-only in the app, matching the API. BGG
sync is read-only; the app never writes to BoardGameGeek.

## OpenTelemetry

The app exports OTLP/HTTP JSON spans for startup, screen views, API requests,
and play submission. API requests also carry a W3C `traceparent`, so server
traces link to the mobile request.

Configuration is compiled in at build time:

```sh
flutter build apk --release \
  --dart-define=OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.example.com \
  --dart-define=OTEL_EXPORTER_OTLP_HEADERS='Authorization=Bearer%20TOKEN' \
  --dart-define=OTEL_SERVICE_NAME=the-shelf-android \
  --dart-define=OTEL_SERVICE_VERSION=1.0.0
```

Headers are comma-separated `key=value` pairs, URL-decoded before use. The
endpoint may be the base URL or the full `/v1/traces` URL. Leave the endpoint
empty to disable export.

Do not bake a long-lived secret into a publicly distributed APK — anyone can
extract it. Build a private APK for authenticated OTLP, or point at a
collector that accepts the app without a secret on your network.

## Umami

```sh
flutter build apk --release \
  --dart-define=UMAMI_URL=https://umami.example.com \
  --dart-define=UMAMI_WEBSITE_ID=xxxxxxxx
```

The app sends `POST /api/send` with a strict allowlist:

| Event | Property | Allowed values |
| --- | --- | --- |
| `screen_view` | `screen` | `shelf`, `game_detail`, `plays`, `stats`, `settings` |
| `shelf_search` | `has_query` | boolean |
| `play_submitted` | `mode` | `create`, `edit` |
| `sync_started` | — | — |
| `theme_changed` | `theme` | `system`, `light`, `dark` |

Game names, notes, player names, search text, tokens, and server URLs can
never be sent. Telemetry is disabled when the dart-defines are absent.

## Development

```sh
cd mobile
flutter pub get
flutter analyze
flutter test
flutter run
```

The app uses Riverpod 3, `dio`, `flutter_secure_storage`, and
`shared_preferences`. Source lives under `mobile/lib/src`:

```text
api/          HTTP client and tolerant DTOs
auth/         secure storage and auth controller
screens/      onboarding, shelf, game, plays, form, stats, settings
telemetry/    OTLP/HTTP and Umami exporters
theme/        warm editorial tokens and Material 3 theme
widgets/      shared cards, covers, pickers, play rows
```

CI runs `flutter analyze`, `flutter test`, and a release APK build for changes
under `mobile/`. The main release workflow attaches
`the-shelf-<version>-android.apk` to the GitHub release.
