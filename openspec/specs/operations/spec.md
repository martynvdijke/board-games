# operations Specification

## Purpose
TBD - created by archiving change add-board-games-v1. Update Purpose after archive.
## Requirements
### Requirement: The application SHALL run as a single container

The project SHALL provide a multi-stage Dockerfile that builds frontend assets and the Go binary, runs as a non-root user, exposes the application port, and stores persistent data under `/data`. A Compose example SHALL mount the data volume and configure the service for a reverse proxy.

#### Scenario: Container start
- **WHEN** the image is started with a valid configuration and an empty `/data`
- **THEN** the application SHALL create its SQLite database and serve the login page

#### Scenario: Non-root runtime
- **WHEN** the container is inspected
- **THEN** the main process SHALL run as a non-root user with write access only to `/data`

#### Scenario: Restart persistence
- **WHEN** the container is restarted with the same volume
- **THEN** users, sessions, games, and plays SHALL remain present

### Requirement: Configuration SHALL come from the environment

The application SHALL read configuration from environment variables, validate required values at startup, and fail with a readable message when configuration is invalid. Secrets SHALL NOT be written to logs.

#### Scenario: Missing required configuration
- **WHEN** `BASE_URL` or `OIDC_ISSUER` is missing in production mode
- **THEN** the process SHALL exit with a clear explanation of the missing value

#### Scenario: Secret redaction
- **WHEN** the application logs its effective configuration
- **THEN** client secrets, session secrets, and tokens SHALL be redacted

#### Scenario: Configurable app identity
- **WHEN** `APP_NAME` is set
- **THEN** the interface and page titles SHALL use it

### Requirement: The application SHALL expose health and metrics endpoints

The application SHALL provide `/healthz` for liveness, `/readyz` for readiness including database access, and `/metrics` in Prometheus format when metrics are enabled. Health endpoints SHALL NOT require authentication and SHALL NOT expose secrets.

#### Scenario: Healthy instance
- **WHEN** the process is running and the database responds
- **THEN** `/healthz` and `/readyz` SHALL return `200 OK`

#### Scenario: Database unavailable
- **WHEN** the database cannot be reached
- **THEN** `/readyz` SHALL return a failure status
- **AND** `/healthz` SHALL continue to report process liveness

#### Scenario: Metrics disabled
- **WHEN** metrics are disabled by configuration
- **THEN** `/metrics` SHALL return `404 Not Found`

### Requirement: The application SHALL support reverse-proxy deployment

The application SHALL bind to an internal address, honour forwarded headers only from configured proxy ranges, use `BASE_URL` for redirects and absolute URLs, and mark cookies secure when the public URL uses HTTPS. TLS SHALL remain the proxy's responsibility.

#### Scenario: Forwarded request
- **WHEN** a request arrives from a trusted proxy with forwarded host and proto headers
- **THEN** generated absolute URLs SHALL use the public scheme and host

#### Scenario: Untrusted forwarded headers
- **WHEN** forwarded headers arrive from an address outside the trusted ranges
- **THEN** the application SHALL ignore them

#### Scenario: OIDC redirect URI
- **WHEN** the application starts
- **THEN** the effective OIDC redirect URI derived from `BASE_URL` SHALL be logged for configuration verification

### Requirement: The database SHALL be migrated automatically and backed up safely

The application SHALL apply embedded, forward-only SQL migrations at startup inside a transaction, record applied versions, and refuse to start if a migration fails. Documentation SHALL describe a consistent SQLite backup procedure.

#### Scenario: Fresh database
- **WHEN** the application starts against an empty data directory
- **THEN** all migrations SHALL be applied and the schema version recorded

#### Scenario: Already migrated
- **WHEN** the application restarts with an up-to-date database
- **THEN** no migration SHALL be re-applied
- **AND** existing data SHALL remain intact

#### Scenario: Failed migration
- **WHEN** a migration fails
- **THEN** the transaction SHALL roll back
- **AND** the process SHALL exit with an actionable error

### Requirement: The application SHALL emit OpenTelemetry telemetry

When OTLP configuration is present, the application SHALL export traces, metrics, and structured logs with service metadata. HTTP requests, BGG calls, sync phases, and database hot paths SHALL be traced. When telemetry is disabled, the application SHALL run without attempting network export.

#### Scenario: Tracing enabled
- **WHEN** an OTLP endpoint is configured
- **THEN** HTTP and sync operations SHALL produce traces with correlated logs

#### Scenario: Telemetry disabled
- **WHEN** no OTLP endpoint is configured
- **THEN** the application SHALL start normally without export errors

#### Scenario: Sensitive data
- **WHEN** telemetry is emitted
- **THEN** tokens, session cookies, OIDC secrets, and personal notes SHALL NOT be included

### Requirement: Analytics SHALL be optional and privacy-friendly

When Umami is configured, HTML pages SHALL load the self-hosted Umami script and privacy-friendly page events. When it is not configured, no analytics script SHALL be emitted. Event properties SHALL NOT include game titles, notes, player names, or tokens.

#### Scenario: Umami configured
- **WHEN** `UMAMI_URL` and `UMAMI_WEBSITE_ID` are set
- **THEN** pages SHALL include the analytics script once

#### Scenario: Umami absent
- **WHEN** Umami configuration is absent
- **THEN** no analytics script or tracking request SHALL be emitted

#### Scenario: No personal data
- **WHEN** a page event is recorded
- **THEN** it SHALL contain only non-identifying interaction data

