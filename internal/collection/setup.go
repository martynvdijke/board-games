package collection

import (
	"context"
	"database/sql"
	"errors"

	"boardgames/internal/db"
)

// MaxSetupImages bounds a single game's setup guide so uploads cannot grow
// without limit. It is intentionally generous: 40 steps is far more than a
// teach-in needs while still protecting storage.
const MaxSetupImages = 40

// setupImages loads a user's setup photos for one game in display order.
func (r *Repo) setupImages(ctx context.Context, userID string, gameID int64) ([]SetupImage, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,filename,content_type,caption,created_at FROM game_setup_images WHERE user_id=? AND game_id=? ORDER BY position,created_at,id`, userID, gameID)
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
		img.Step = len(out) + 1
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

// MoveSetupImage moves one owned setup photo one slot earlier (delta -1) or
// later (delta +1), then rewrites every position for that game as a contiguous
// 0..n-1 sequence so ordering stays stable across later uploads and deletes.
// It reports false when the photo is already at the requested edge. The photo
// must belong to the supplied user and game; anything else is ErrNotFound.
func (r *Repo) MoveSetupImage(ctx context.Context, userID string, gameID int64, imageID string, delta int) (bool, error) {
	if delta != -1 && delta != 1 {
		return false, ErrNotFound
	}
	moved := false
	err := db.WithTx(ctx, r.DB, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM game_setup_images WHERE user_id=? AND game_id=? ORDER BY position,created_at,id`, userID, gameID)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		index := -1
		for i, id := range ids {
			if id == imageID {
				index = i
				break
			}
		}
		if index < 0 {
			return ErrNotFound
		}
		target := index + delta
		if target < 0 || target >= len(ids) {
			return nil
		}
		ids[index], ids[target] = ids[target], ids[index]
		for position, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE game_setup_images SET position=? WHERE user_id=? AND game_id=? AND id=?`, position, userID, gameID, id); err != nil {
				return err
			}
		}
		moved = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return moved, nil
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
