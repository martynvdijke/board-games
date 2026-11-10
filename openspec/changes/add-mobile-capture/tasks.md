## 1. API idempotency

- [ ] 1.1 Add nullable `plays.idempotency_key` with a partial unique index on `(user_id, idempotency_key)`
- [ ] 1.2 Honour `Idempotency-Key` on play creation, returning the existing play on replay
- [ ] 1.3 Return a deterministic conflict when the same key is used with different content
- [ ] 1.4 Confirm behaviour is unchanged when no key is supplied
- [ ] 1.5 Update `docs/api.md` and the OpenAPI document

## 2. Offline queue

- [ ] 2.1 Add `drift` and a queue table with body, key, attempts, next attempt and last error
- [ ] 2.2 Generate a client identifier per queued play and send it as the idempotency key
- [ ] 2.3 Implement exponential backoff with jitter and a hard retry cap that marks the row as needing attention
- [ ] 2.4 Trigger sync on foreground, pull-to-refresh and after successful sign-in
- [ ] 2.5 Discard the queue on sign-out after warning the user when items are pending
- [ ] 2.6 Encrypt the queue at rest using the existing secure storage mechanism

## 3. Sync visibility

- [ ] 3.1 Show a pending indicator in the plays list and a pending count in settings
- [ ] 3.2 Surface retry exhaustion as an actionable row rather than a silent failure
- [ ] 3.3 Report a server/local divergence once when the server record wins

## 4. Barcode capture

- [ ] 4.1 Add the barcode scanning dependency and a capture entry point that does not prompt at launch
- [ ] 4.2 Resolve a scanned code to a game and show details for confirmation before adding
- [ ] 4.3 Fall back to text search for unknown codes and explain the already-owned case
- [ ] 4.4 Keep manual search fully usable when the camera is denied or unavailable

## 5. Verification

- [ ] 5.1 Test that a replayed create with the same key yields one play
- [ ] 5.2 Test that a key reused with different content returns a conflict
- [ ] 5.3 Test queue retry progression and the exhaustion state
- [ ] 5.4 Test that signing out with pending plays discards them after a warning
- [ ] 5.5 Widget test the pending indicator and the barcode fallback path
- [ ] 5.6 Update `docs/mobile.md`
