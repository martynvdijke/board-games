## ADDED Requirements

### Requirement: Users SHALL authenticate through OIDC

The system SHALL authenticate users with the OpenID Connect Authorization Code flow using PKCE. It SHALL discover endpoints from the configured issuer, validate `state` and `nonce`, verify the ID token signature, issuer, audience, and expiry, and reject any callback that fails validation.

#### Scenario: Successful login
- **WHEN** an unauthenticated visitor starts login and completes authentication at the configured OIDC provider
- **THEN** the system SHALL create or update the local user record and establish a session
- **AND** the visitor SHALL be redirected to the originally requested page or the shelf overview

#### Scenario: Invalid callback state
- **WHEN** a callback arrives with a missing, expired, or mismatched `state` or `nonce`
- **THEN** the system SHALL reject the login with a generic authentication error
- **AND** no session SHALL be created

#### Scenario: Provider unavailable
- **WHEN** the OIDC discovery or token endpoint is unreachable
- **THEN** the login page SHALL show a readable error and a retry action
- **AND** the failure SHALL be logged without exposing tokens or secrets

### Requirement: Access SHALL be limited to permitted identities

The system SHALL assign the first successfully authenticated user the `admin` role and later users the `member` role. When an allowlist is configured, only identities whose verified email or email domain matches SHALL be allowed to create a local user or session.

#### Scenario: First user becomes admin
- **WHEN** the first user completes OIDC login on a fresh database
- **THEN** that user SHALL have the `admin` role

#### Scenario: Disallowed identity
- **WHEN** a user authenticates successfully at the provider but their email is not permitted by the configured allowlist
- **THEN** the system SHALL refuse to create a session
- **AND** the response SHALL NOT reveal whether the email exists locally

#### Scenario: Unverified email policy
- **WHEN** the provider reports an email as unverified and the allowlist relies on email
- **THEN** the system SHALL refuse access

### Requirement: Sessions SHALL be secure and database-backed

Sessions SHALL use a cryptographically random token delivered in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie. Only a hash of the session token SHALL be stored. Sessions SHALL expire after the configured TTL, be revocable on logout, and be invalidated when the user record is removed.

#### Scenario: Session expiry
- **WHEN** a session's expiry time has passed
- **THEN** the next request SHALL be treated as unauthenticated
- **AND** the expired session row SHALL be removed

#### Scenario: Logout
- **WHEN** a user logs out
- **THEN** the session row SHALL be deleted and the cookie cleared
- **AND** replaying the old cookie SHALL NOT authenticate

#### Scenario: Cookie flags
- **WHEN** the application runs with a configured `BASE_URL` using HTTPS
- **THEN** the session cookie SHALL be marked `Secure`, `HttpOnly`, and `SameSite=Lax`

### Requirement: Unsafe requests SHALL be protected against CSRF

Cookie-authenticated state-changing requests SHALL require a valid per-session CSRF token or an equivalent double-submit check, and SHALL reject requests whose `Origin` does not match the configured base URL. Bearer-token requests SHALL be exempt from cookie CSRF checks.

#### Scenario: Missing CSRF token
- **WHEN** a cookie-authenticated form posts without a valid CSRF token
- **THEN** the request SHALL be rejected with `403 Forbidden`
- **AND** no state SHALL change

#### Scenario: Cross-origin post
- **WHEN** a state-changing request arrives with an `Origin` that does not match `BASE_URL`
- **THEN** the request SHALL be rejected

### Requirement: Users SHALL manage scoped personal API tokens

The system SHALL let an authenticated user create, list, and revoke personal API tokens. A token SHALL be shown in full exactly once, stored only as a hash with a display prefix, carry one or more scopes (`read`, `write`, `trmnl`), and record its last use time. Revoked tokens SHALL fail authentication immediately.

#### Scenario: Create token
- **WHEN** a user creates a token named "Android" with `read` and `write` scopes
- **THEN** the full token SHALL be displayed once
- **AND** the stored record SHALL contain only a hash and prefix

#### Scenario: Revoke token
- **WHEN** a user revokes a token
- **THEN** subsequent requests using that token SHALL return `401 Unauthorized`

#### Scenario: Insufficient scope
- **WHEN** a token with only `read` scope calls a write endpoint
- **THEN** the system SHALL return `403 Forbidden`
- **AND** the response SHALL identify the missing scope without disclosing internals

#### Scenario: TRMNL scope
- **WHEN** a token with only `trmnl` scope calls the TRMNL feed
- **THEN** the request SHALL succeed
- **AND** the same token SHALL NOT access collection or play endpoints

### Requirement: API requests SHALL accept supported bearer credentials

The API SHALL authenticate requests using a personal API token, and MAY accept a provider-issued JWT access token validated against the issuer's JWKS. When both a session cookie and a bearer token are present, the bearer token SHALL determine the identity and scopes.

#### Scenario: Bearer token authentication
- **WHEN** a client sends `Authorization: Bearer <valid-token>`
- **THEN** the request SHALL be authenticated as the token's owner with the token's scopes

#### Scenario: Expired JWT
- **WHEN** a client sends a JWT whose signature, issuer, audience, or expiry is invalid
- **THEN** the API SHALL return `401 Unauthorized`

### Requirement: User data SHALL be isolated by user

Every collection, play, setting, session, and token query SHALL be scoped to the authenticated user's identifier. A member SHALL NOT read or modify another user's shelf, plays, settings, or tokens.

#### Scenario: Cross-user access
- **WHEN** an authenticated user requests another user's play by identifier
- **THEN** the system SHALL return `404 Not Found`
- **AND** no data from the other user SHALL be returned

#### Scenario: Admin boundary
- **WHEN** an admin views instance information
- **THEN** the admin SHALL NOT automatically see another user's private notes or plays in v1
