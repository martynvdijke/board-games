ALTER TABLE plays ADD COLUMN client_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_plays_user_client_id
    ON plays(user_id, client_id)
    WHERE client_id IS NOT NULL AND client_id <> '';
