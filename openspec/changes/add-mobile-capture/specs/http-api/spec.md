## ADDED Requirements

### Requirement: Idempotent play creation
Play creation SHALL accept a client-supplied idempotency key and SHALL NOT create a second play for a repeated request with the same key.

#### Scenario: First request
- **WHEN** a client posts a play with an idempotency key it has not used
- **THEN** the play is created and the key is stored with it

#### Scenario: Replay
- **WHEN** a client posts the same play with the same idempotency key
- **THEN** the response describes the play that already exists and no new play is created

#### Scenario: Key reuse with different content
- **WHEN** a client posts different play content under an existing idempotency key
- **THEN** the API returns a deterministic conflict in the standard error envelope rather than overwriting

#### Scenario: No key supplied
- **WHEN** a client posts a play without an idempotency key
- **THEN** behaviour is unchanged and the play is created as before
