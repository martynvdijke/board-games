## Why

Shelf currently ships signed Android release APKs through GitHub Releases only. Android users have no Play Store listing, so they get no automatic updates, no staged rollouts, and no store-level crash reporting. Every release also depends on a manual APK download. At the same time the app has no emulator-based regression test, so a release can pass unit tests while failing to start on a real device.

## What Changes

- Add `.github/workflows/play-store.yaml`, a reusable workflow that builds a signed Android App Bundle from the release tag and uploads it to Google Play.
- Wire a `play-store` job into `.github/workflows/release.yaml` so every published release is promoted to Play automatically.
- Upload the bundle on the `internal` track with `completed` status by default, configurable per repository through `GOOGLE_PLAY_TRACK` and `GOOGLE_PLAY_RELEASE_STATUS`.
- Use `ANDROID_PACKAGE_NAME`, `GOOGLE_PLAY_TRACK`, `GOOGLE_PLAY_RELEASE_STATUS` repository variables and the `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` secret for Play authentication, keeping signing uniform with the existing `ANDROID_*` secrets.
- Add `.github/workflows/android-e2e.yaml` plus a `mobile/integration_test/app_e2e_test.dart` boot test that launches the app on an Android 34 emulator.
- Keep the existing Go backend test coverage (`go test ./...` in CI) as the backend gate.

### Non-goals

- iOS/App Store distribution.
- Automated store listing assets (screenshots, descriptions, localized metadata) beyond the bundle upload.
- Changing the existing GitHub Release APK attachment.
- Automated Play review submission of a production rollout beyond the configured track/status.

## Capabilities

### New Capabilities

- `play-store-release`: Signed App Bundle build and Google Play upload driven by semantic-release versions.
- `android-emulator-e2e`: Emulator-based integration test that verifies the app boots and renders its first screen.

### Modified Capabilities

- None.

## Impact

- New `.github/workflows/play-store.yaml`.
- New `play-store` job in `.github/workflows/release.yaml`.
- New `.github/workflows/android-e2e.yaml`.
- New `mobile/integration_test/app_e2e_test.dart` and `integration_test` dev dependency in `mobile/pubspec.yaml`.
- Requires repository variables `ANDROID_PACKAGE_NAME`, `GOOGLE_PLAY_TRACK`, `GOOGLE_PLAY_RELEASE_STATUS` and secret `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`.
