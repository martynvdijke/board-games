CREATE TABLE IF NOT EXISTS locations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (user_id, normalized_name)
);

ALTER TABLE plays ADD COLUMN location_id INTEGER REFERENCES locations(id);

CREATE INDEX IF NOT EXISTS idx_plays_location_id ON plays(location_id);
