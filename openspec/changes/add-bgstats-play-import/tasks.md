## 1. Data model

- [ ] 1.1 Recreate `plays` with `origin` accepting `import`, preserving all existing rows and indexes
- [ ] 1.2 Add nullable `import_key` with a partial unique index on `(user_id, import_key)`
- [ ] 1.3 Generalise the immutability guard from `origin = 'bgg'` to any non-local origin
- [ ] 1.4 Verify row counts before and after the constraint migration and keep it idempotent

## 2. Parsing and mapping

- [ ] 2.1 Implement CSV parsing with header detection and a candidate mapping for date, game, players, scores, winner, duration, location and notes
- [ ] 2.2 Detect ambiguous date ordering and require an explicit choice
- [ ] 2.3 Report rows that cannot be parsed with line numbers
- [ ] 2.4 Match games by BoardGameGeek id first, then exact normalised name, never fuzzily
- [ ] 2.5 Compute the per-row import key from date, game, sorted players, scores and quantity

## 3. Dry run and commit

- [ ] 3.1 Persist the parsed import session server-side with a short expiry
- [ ] 3.2 Build the preview report with created, duplicate, unmatched and unparsable counts plus player consequences
- [ ] 3.3 Implement batched commit of up to 200 plays per transaction using `db.WithTx`
- [ ] 3.4 Resolve player names to canonical players during commit
- [ ] 3.5 Support the explicit duplicate override that omits the import key
- [ ] 3.6 Report per-batch failures without aborting the whole import

## 4. Interface

- [ ] 4.1 Add the upload, mapping and preview screens with CSRF protection
- [ ] 4.2 Show the parsed date samples in full so a format mistake is visible
- [ ] 4.3 Mark imported plays as not locally editable in the play timeline
- [ ] 4.4 Add test fixtures covering a normal export, duplicates and malformed rows
- [ ] 4.5 Update the README with the supported export format

## 5. Verification

- [ ] 5.1 Test that uploading the same export twice creates no duplicates
- [ ] 5.2 Test that an ambiguous date file cannot proceed without a choice
- [ ] 5.3 Test that an unmatched game is never imported under a guessed game
- [ ] 5.4 Test that editing or deleting an imported play is refused
- [ ] 5.5 Test that a subsequent BoardGameGeek sync leaves imported plays untouched
