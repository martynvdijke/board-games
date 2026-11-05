ALTER TABLE user_games ADD COLUMN setup_notes TEXT NOT NULL DEFAULT '';

CREATE TABLE game_setup_images (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id      INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL,
    caption      TEXT NOT NULL DEFAULT '',
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_setup_images_user_game ON game_setup_images(user_id, game_id, position);
