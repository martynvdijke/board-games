CREATE TABLE games (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    bgg_id         INTEGER NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    sort_name      TEXT NOT NULL DEFAULT '',
    year_published INTEGER,
    description    TEXT NOT NULL DEFAULT '',
    image_url      TEXT NOT NULL DEFAULT '',
    thumbnail_url  TEXT NOT NULL DEFAULT '',
    min_players    INTEGER,
    max_players    INTEGER,
    min_playtime   INTEGER,
    max_playtime   INTEGER,
    min_age        INTEGER,
    weight         REAL,
    bgg_rating     REAL,
    bgg_rank       INTEGER,
    is_expansion   INTEGER NOT NULL DEFAULT 0 CHECK (is_expansion IN (0, 1)),
    fetched_at     TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE INDEX idx_games_name ON games(name COLLATE NOCASE);
CREATE INDEX idx_games_year ON games(year_published);
CREATE INDEX idx_games_weight ON games(weight);

CREATE TABLE categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE
);
CREATE TABLE mechanics (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE
);
CREATE TABLE designers (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE
);
CREATE TABLE publishers (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE
);

CREATE TABLE game_categories (
    game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (game_id, category_id)
);
CREATE INDEX idx_game_categories_category ON game_categories(category_id, game_id);

CREATE TABLE game_mechanics (
    game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mechanic_id INTEGER NOT NULL REFERENCES mechanics(id) ON DELETE CASCADE,
    PRIMARY KEY (game_id, mechanic_id)
);
CREATE INDEX idx_game_mechanics_mechanic ON game_mechanics(mechanic_id, game_id);

CREATE TABLE game_designers (
    game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    designer_id INTEGER NOT NULL REFERENCES designers(id) ON DELETE CASCADE,
    PRIMARY KEY (game_id, designer_id)
);

CREATE TABLE game_publishers (
    game_id      INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    publisher_id INTEGER NOT NULL REFERENCES publishers(id) ON DELETE CASCADE,
    PRIMARY KEY (game_id, publisher_id)
);

CREATE VIRTUAL TABLE games_fts USING fts5(
    name,
    description,
    designers,
    publishers,
    categories,
    mechanics,
    tokenize = 'unicode61 remove_diacritics 2'
);
