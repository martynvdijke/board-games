package plays

import "context"

// PlayCountGap compares BGG's reported play count with the plays the user has
// logged locally. A zero or negative BGG count means BGG has never reported
// plays for the game, which is no signal at all.
type PlayCountGap struct {
	BGG    int
	Logged int
}

// NeedsCatchUp reports whether BGG knows about more plays than are logged.
func (g PlayCountGap) NeedsCatchUp() bool { return g.BGG > 0 && g.BGG > g.Logged }

// Missing is how many plays BGG knows about that the log does not.
func (g PlayCountGap) Missing() int {
	if !g.NeedsCatchUp() {
		return 0
	}
	return g.BGG - g.Logged
}

// ReconcileSummary is the library-wide catch-up.
type ReconcileSummary struct {
	Games   int
	Missing int
}

// SummariseReconcile totals the gaps for games where BGG is ahead. It reports
// false when there is nothing to say, so callers can hide the whole sentence.
func SummariseReconcile(gaps []PlayCountGap) (ReconcileSummary, bool) {
	var s ReconcileSummary
	for _, g := range gaps {
		if !g.NeedsCatchUp() {
			continue
		}
		s.Games++
		s.Missing += g.BGG - g.Logged
	}
	return s, s.Games > 0 && s.Missing > 0
}

// LoggedPlayCount counts the user's own plays of one game. It counts play
// rows, not quantities, to match what BGG reports.
func (r *Repo) LoggedPlayCount(ctx context.Context, userID string, gameID int64) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND game_id=?`, userID, gameID).Scan(&n)
	return n, err
}

// ReconcileGaps returns a gap for every shelf game BGG reports plays for. The
// left join keeps games with no logged plays, which are exactly the ones with
// the widest gap.
func (r *Repo) ReconcileGaps(ctx context.Context, userID string) ([]PlayCountGap, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT ug.bgg_num_plays, COUNT(p.id)
		FROM user_games ug
		LEFT JOIN plays p ON p.game_id=ug.game_id AND p.user_id=ug.user_id
		WHERE ug.user_id=? AND ug.bgg_num_plays>0
		GROUP BY ug.game_id, ug.bgg_num_plays`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlayCountGap
	for rows.Next() {
		var g PlayCountGap
		if err := rows.Scan(&g.BGG, &g.Logged); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
