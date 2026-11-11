CREATE TABLE IF NOT EXISTS year_review_shares (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year            INTEGER NOT NULL,
    token           TEXT NOT NULL UNIQUE,
    include_players INTEGER NOT NULL DEFAULT 0 CHECK (include_players IN (0, 1)),
    created_at      TEXT NOT NULL,
    revoked_at      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_year_review_shares_user_year ON year_review_shares(user_id, year);
