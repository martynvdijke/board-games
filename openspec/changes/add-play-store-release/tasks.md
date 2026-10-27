## 1. Play Store publish workflow

- [x] 1.1 Add `.github/workflows/play-store.yaml` as a `workflow_call` workflow with `version` and optional `build-number` inputs.
- [x] 1.2 Build a signed App Bundle with `flutter build appbundle --release --build-name <version> --build-number <commit-count>`.
- [x] 1.3 Upload the bundle with `r0adkll/upload-google-play`, `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`, `vars.ANDROID_PACKAGE_NAME`, `vars.GOOGLE_PLAY_TRACK`, and `vars.GOOGLE_PLAY_RELEASE_STATUS`.
- [x] 1.4 Upload the `.aab` as a workflow artifact for auditability.

## 2. Release integration

- [x] 2.1 Add a `play-store` job to `.github/workflows/release.yaml` that needs `release` and forwards `new_release_version`.
- [x] 2.2 Gate the job on a non-empty version so `[skip ci]` and no-release runs do not attempt an upload.

## 3. Emulator E2E

- [x] 3.1 Add `.github/workflows/android-e2e.yaml` using `reactivecircus/android-emulator-runner` (API 34, `google_apis`, x86_64, `pixel_6`).
- [x] 3.2 Add `mobile/integration_test/app_e2e_test.dart` boot test and the `integration_test` dev dependency.
- [ ] 3.3 Deepen the E2E test with an authenticated flow against the Go backend (follow-up change).

## 4. Verification

- [x] 4.1 Validate the new workflows with `actionlint`.
- [x] 4.2 Validate the OpenSpec change with `openspec validate --strict`.
- [ ] 4.3 Confirm the first Play upload after the service account is configured.
