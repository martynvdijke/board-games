package collection

import (
	"context"
	"database/sql"
	"errors"

	"boardgames/internal/db"
)

const MaxSetupImages = 12

// setupImages loads a user's setup photos for one game in display order.
func (r *Repo) setupImages(ctx context.Context, userID string, gameID int64) ([]SetupImage, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,filename,content_type,caption,created_at FROM game_setup_images WHERE user_id=? AND game_id=? ORDER BY position,created_at`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SetupImage
	for rows.Next() {
		var img SetupImage
		if err := rows.Scan(&img.ID, &img.Filename, &img.ContentType, &img.Caption, &img.CreatedAt); err != nil {
			return nil, err
		}
		img.URL = "/media/setup/" + img.ID
		out = append(out, img)
	}
	return out, rows.Err()
}

// AddSetupImage records an uploaded setup photo. It only succeeds when the
// game exists on the supplied user's shelf.
func (r *Repo) AddSetupImage(ctx context.Context, userID string, gameID int64, id, filename, contentType, caption string) error {
	res, err := r.DB.ExecContext(ctx, `INSERT INTO game_setup_images (id,user_id,game_id,filename,content_type,caption,position,created_at)
		SELECT ?,?,?,?,?,?,COALESCE((SELECT MAX(position)+1 FROM game_setup_images WHERE user_id=? AND game_id=?),0),?
		WHERE EXISTS (SELECT 1 FROM user_games WHERE user_id=? AND game_id=?)`,
		id, userID, gameID, filename, contentType, caption, userID, gameID, db.Now(), userID, gameID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountSetupImages returns how many setup photos the user has for a game.
func (r *Repo) CountSetupImages(ctx context.Context, userID string, gameID int64) (int, error) {
	var n int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_setup_images WHERE user_id=? AND game_id=?`, userID, gameID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// SetupImage returns one owned setup photo.
func (r *Repo) SetupImage(ctx context.Context, userID, imageID string) (*SetupImage, error) {
	var img SetupImage
	err := r.DB.QueryRowContext(ctx, `SELECT id,filename,content_type,caption,created_at FROM game_setup_images WHERE user_id=? AND id=?`, userID, imageID).
		Scan(&img.ID, &img.Filename, &img.ContentType, &img.Caption, &img.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	img.URL = "/media/setup/" + img.ID
	return &img, nil
}

// DeleteSetupImage removes an owned setup photo and returns its storage filename.
func (r *Repo) DeleteSetupImage(ctx context.Context, userID, imageID string) (string, error) {
	var filename string
	err := r.DB.QueryRowContext(ctx, `DELETE FROM game_setup_images WHERE user_id=? AND id=? RETURNING filename`, userID, imageID).Scan(&filename)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return filename, nil
}
