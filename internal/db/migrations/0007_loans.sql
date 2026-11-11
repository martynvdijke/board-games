CREATE TABLE IF NOT EXISTS loans (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id       INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    borrower      TEXT NOT NULL,
    contact       TEXT NOT NULL DEFAULT '',
    lent_at       TEXT NOT NULL,
    due_at        TEXT NOT NULL DEFAULT '',
    returned_at   TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    closed_reason TEXT NOT NULL DEFAULT '',
    notified_at   TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_loans_open_unique ON loans(user_id, game_id) WHERE returned_at = '';
CREATE INDEX IF NOT EXISTS idx_loans_user_open ON loans(user_id, lent_at) WHERE returned_at = '';
CREATE INDEX IF NOT EXISTS idx_loans_game ON loans(game_id);
