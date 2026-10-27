## Context

Shelf is a Flutter Android app (`mobile/`, application ID `xyz.vandijke.shelf`) released from `.github/workflows/release.yaml` with semantic-release. The release job publishes a GitHub Release and a signed APK. Android signing already uses the shared `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, and `ANDROID_KEY_PASSWORD` secrets, and every repository in this fleet uses the same upload keystore. Google Play requires an App Bundle (`.aab`) with a monotonically increasing version code, uploaded with a service account that has release permissions.

## Goals / Non-Goals

**Goals:**
- Publish every semantic-release version to Google Play without a manual step.
- Keep signing identical to the existing APK pipeline so the Play upload key stays the same as the F-Droid/GitHub key.
- Guarantee the version code always increases across releases.
- Prove on every pull request that the app still boots on an emulator.

**Non-Goals:**
- Managing Play listing metadata, screenshots, or translations from the repository.
- Automating promotion beyond the configured track (`internal` by default).
- Replacing the GitHub Release APK asset.

## Decisions

1. **Reusable workflow, called from the release workflow** — `play-store.yaml` is `workflow_call`-only and receives the released version. The `play-store` job sits after `release` in `.github/workflows/release.yaml` and runs only when `needs.release.outputs.new_release_version != ''`. This works with a `GITHUB_TOKEN`-driven release, where tag-push or `release` event triggers would be suppressed.
2. **Build the bundle from the release tag** — the workflow checks out `v<version>` so the AAB matches the published tag rather than the tip of the branch.
3. **Version code from the commit count** — `--build-number=$(git rev-list --count HEAD)` produces a strictly increasing Play version code from a shallow-history-free checkout, while `--build-name` carries the semantic version. An optional `build-number` input allows a manual override for re-runs.
4. **Repository variables for per-app settings** — `ANDROID_PACKAGE_NAME`, `GOOGLE_PLAY_TRACK`, and `GOOGLE_PLAY_RELEASE_STATUS` are variables (not secrets) so the track and package are visible in workflow logs, with `internal`/`completed` fallbacks in the workflow.
5. **Service account JSON as a secret** — `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` is the single Play credential, pushed from the central homelab secrets file.
6. **Emulator boot test instead of a deep flow test** — the E2E test launches `main()` on an API 34 `google_apis` x86_64 emulator and asserts a `MaterialApp`/`CupertinoApp` mounts without exceptions. It is deliberately server-independent so it runs on pull requests without seeding a database.

## Risks / Trade-offs

- **[Version code collision]** -> A manual re-run of the same tag reuses the same commit count; pass the `build-number` input with a higher value, or bump the tag.
- **[First Play upload needs manual listing]** -> Google Play requires the app record, package name, and at least one track to exist before API uploads succeed. The first bundle may need to be uploaded manually once; the workflow then takes over.
- **[Boot test is shallow]** -> It catches startup crashes and missing widgets, not feature regressions. Deeper flows belong in follow-up change proposals.
- **[Package ownership]** -> The package name must be controlled by the Play developer account that owns `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`.
