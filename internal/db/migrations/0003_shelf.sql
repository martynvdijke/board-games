CREATE TABLE user_games (
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    owned          INTEGER NOT NULL DEFAULT 0 CHECK (owned IN (0, 1)),
    wishlist       INTEGER NOT NULL DEFAULT 0 CHECK (wishlist IN (0, 1)),
    want_to_play   INTEGER NOT NULL DEFAULT 0 CHECK (want_to_play IN (0, 1)),
    preordered     INTEGER NOT NULL DEFAULT 0 CHECK (preordered IN (0, 1)),
    for_trade      INTEGER NOT NULL DEFAULT 0 CHECK (for_trade IN (0, 1)),
    favorite       INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1)),
    user_rating    REAL CHECK (user_rating IS NULL OR (user_rating >= 1 AND user_rating <= 10 AND user_rating * 2 = CAST(user_rating * 2 AS INTEGER))),
    notes          TEXT NOT NULL DEFAULT '',
    acquisition_date TEXT,
    price_paid     INTEGER,
    location       TEXT NOT NULL DEFAULT '',
    condition      TEXT NOT NULL DEFAULT '',
    bgg_num_plays  INTEGER NOT NULL DEFAULT 0,
    added_at       TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    PRIMARY KEY (user_id, game_id)
);
CREATE INDEX idx_user_games_game ON user_games(game_id);
CREATE INDEX idx_user_games_owned ON user_games(user_id, owned);
CREATE INDEX idx_user_games_wishlist ON user_games(user_id, wishlist);

CREATE TABLE plays (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id          INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    played_at        TEXT NOT NULL,
    location         TEXT NOT NULL DEFAULT '',
    duration_minutes INTEGER CHECK (duration_minutes IS NULL OR duration_minutes >= 0),
    quantity         INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 1),
    notes            TEXT NOT NULL DEFAULT '',
    origin           TEXT NOT NULL DEFAULT 'local' CHECK (origin IN ('local', 'bgg')),
    bgg_play_id      INTEGER,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);
CREATE INDEX idx_plays_user_date ON plays(user_id, played_at DESC);
CREATE INDEX idx_plays_game ON plays(game_id);
CREATE UNIQUE INDEX idx_plays_bgg_unique ON plays(user_id, bgg_play_id) WHERE bgg_play_id IS NOT NULL;

CREATE TABLE play_players (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    play_id       TEXT NOT NULL REFERENCES plays(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    score         TEXT NOT NULL DEFAULT '',
    is_winner     INTEGER NOT NULL DEFAULT 0 CHECK (is_winner IN (0, 1)),
    is_first_play INTEGER NOT NULL DEFAULT 0 CHECK (is_first_play IN (0, 1)),
    position      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_play_players_play ON play_players(play_id);
CREATE INDEX idx_play_players_name ON play_players(name COLLATE NOCASE);

CREATE TABLE sync_runs (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'partial', 'failed')),
    phase            TEXT NOT NULL DEFAULT 'queued',
    collection_count INTEGER NOT NULL DEFAULT 0,
    things_count     INTEGER NOT NULL DEFAULT 0,
    plays_count      INTEGER NOT NULL DEFAULT 0,
    error            TEXT NOT NULL DEFAULT '',
    trigger          TEXT NOT NULL DEFAULT 'manual' CHECK (trigger IN ('manual', 'scheduled', 'api')),
    started_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    finished_at      TEXT
);
CREATE INDEX idx_sync_runs_user_started ON sync_runs(user_id, started_at DESC);
CREATE INDEX idx_sync_runs_status ON sync_runs(status);
