ALTER TABLE users ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0 CHECK (is_demo IN (0,1));
ALTER TABLE user_settings ADD COLUMN analytics_opt_out INTEGER NOT NULL DEFAULT 0 CHECK (analytics_opt_out IN (0,1));
