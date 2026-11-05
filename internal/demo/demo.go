// Package demo creates the optional public demonstration account used for
// app-store review. It only ever owns a user row flagged is_demo=1.
package demo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"boardgames/internal/auth"
	"boardgames/internal/config"
	"boardgames/internal/db"
)

// Service ensures the demo account, its personal token, and sample content.
type Service struct {
	DB     *sql.DB
	Auth   *auth.Manager
	Config *config.Config
}

func New(database *sql.DB, a *auth.Manager, cfg *config.Config) *Service {
	return &Service{DB: database, Auth: a, Config: cfg}
}

// Ensure is idempotent across restarts. It refuses to take over an existing
// non-demo account that happens to use the configured demo email.
func (s *Service) Ensure(ctx context.Context) error {
	if !s.Config.DemoUserEnabled {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE is_demo=1)`); err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM api_tokens WHERE user_id IN (SELECT id FROM users WHERE is_demo=1)`); err != nil {
			return err
		}
		slog.Info("demo account disabled")
		return nil
	}
	if s.Config.DemoUserToken == "" {
		return errors.New("DEMO_USER_TOKEN is required when the demo account is enabled")
	}

	var userID string
	err := db.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		var collisionID string
		var collisionDemo int
		err := tx.QueryRowContext(ctx, `SELECT id,is_demo FROM users WHERE email=?`, s.Config.DemoUserEmail).
			Scan(&collisionID, &collisionDemo)
		switch {
		case err == nil && collisionDemo == 0:
			return errors.New("the configured demo email already belongs to a non-demo account; refusing to take it over")
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return err
		}

		var existingEmail string
		err = tx.QueryRowContext(ctx, `SELECT id,email FROM users WHERE is_demo=1 ORDER BY created_at ASC, id ASC LIMIT 1`).
			Scan(&userID, &existingEmail)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			userID = newID()
			now := db.Now()
			_, err = tx.ExecContext(ctx, `INSERT INTO users(id,email,name,avatar_url,role,is_demo,created_at,updated_at,last_login_at)
				VALUES(?,?,?,'','member',1,?,?,NULL)`, userID, s.Config.DemoUserEmail, s.Config.DemoUserName, now, now)
			if err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE is_demo=1 AND id<>?`, userID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET email=?,name=?,role='member',is_demo=1,updated_at=? WHERE id=?`,
				s.Config.DemoUserEmail, s.Config.DemoUserName, db.Now(), userID); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO user_settings(user_id, timezone, updated_at) VALUES(?,?,?)
			ON CONFLICT(user_id) DO NOTHING`, userID, s.Config.Timezone, db.Now())
		return err
	})
	if err != nil {
		return err
	}

	if err := s.Auth.EnsurePersonalToken(ctx, userID, "Play Store demo", s.Config.DemoUserToken, []string{"read", "write"}); err != nil {
		return fmt.Errorf("ensure demo token: %w", err)
	}
	if err := s.seed(ctx, userID); err != nil {
		return fmt.Errorf("seed demo content: %w", err)
	}
	slog.Info("demo account ready", "user_id", userID)
	return nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
