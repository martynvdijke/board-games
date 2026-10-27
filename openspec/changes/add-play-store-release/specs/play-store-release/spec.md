## ADDED Requirements

### Requirement: Play Store bundle publishing
The release pipeline SHALL build a signed Android App Bundle for the released version and upload it to Google Play using a service account credential.

#### Scenario: A new release publishes to Play
- **WHEN** semantic-release publishes a new version and the `release` job succeeds with a non-empty `new_release_version`
- **THEN** the `play-store` job builds the bundle from tag `v<version>` and uploads it to the track from `GOOGLE_PLAY_TRACK` (default `internal`)

#### Scenario: No release was published
- **WHEN** a run completes without a new semantic-release version
- **THEN** the `play-store` job is skipped and no Play upload is attempted

#### Scenario: Missing Play credentials
- **WHEN** `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` is not configured
- **THEN** the upload step fails with a clear error instead of silently skipping distribution

### Requirement: Monotonic version code
The uploaded bundle SHALL carry a version code that increases with every release.

#### Scenario: Version code derived from history
- **WHEN** the bundle is built without an explicit `build-number` input
- **THEN** the version code is the git commit count of the checked-out release tag, and the version name is the semantic version

### Requirement: Signing parity with existing releases
The App Bundle SHALL be signed with the same upload key used for the GitHub Release APK.

#### Scenario: Shared signing secrets
- **WHEN** the bundle build runs
- **THEN** it materializes `upload-keystore.jks` and `key.properties` from the shared `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, and `ANDROID_KEY_PASSWORD` secrets
