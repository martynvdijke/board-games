package plays

import (
	"context"
	"database/sql"
	"strings"
)

// RecentPlayers returns the names this user has played with most recently,
// de-duplicated case-insensitively and trimmed of surrounding whitespace.
// It powers the tick-list of familiar names on the play form; it is a read of
// existing play data, not a friends list.
func (r *Repo) RecentPlayers(ctx context.Context, userID string, limit int) ([]string, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT MAX(pp.name) AS name
		FROM play_players pp
		JOIN plays p ON p.id = pp.play_id
		WHERE p.user_id = ? AND TRIM(pp.name) <> ''
		GROUP BY lower(trim(pp.name))
		ORDER BY MAX(p.played_at) DESC, name COLLATE NOCASE
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name.Valid && strings.TrimSpace(name.String) != "" {
			out = append(out, strings.TrimSpace(name.String))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
