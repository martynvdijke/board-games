## ADDED Requirements

### Requirement: Emulator boot smoke test
Every pull request SHALL run an Android emulator integration test that launches the app and verifies the first screen renders.

#### Scenario: Pull request runs the emulator test
- **WHEN** a pull request is opened or updated
- **THEN** `.github/workflows/android-e2e.yaml` starts an API 34 `google_apis` x86_64 emulator and runs `flutter test integration_test`

#### Scenario: Startup regression fails the check
- **WHEN** the app throws during startup or never mounts its root app widget
- **THEN** the integration test fails and the workflow reports a failure

### Requirement: Server-independent E2E test
The boot test SHALL NOT depend on a running backend so it can execute on every pull request.

#### Scenario: No backend available
- **WHEN** the emulator test runs without the Go server running
- **THEN** the app still mounts its first screen and the test passes
