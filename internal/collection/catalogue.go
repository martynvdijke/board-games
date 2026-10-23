package collection

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"boardgames/internal/bgg"
	"boardgames/internal/db"
)

// UpsertCollectionItem writes shared catalogue basics and the user's BGG shelf
// state. Local-only fields (notes, price, location, condition, favorite) are
// never overwritten.
func UpsertCollectionItem(ctx context.Context, tx *sql.Tx, userID string, item bgg.CollectionItem) (int64, error) {
	now := db.Now()
	rank := 0
	for _, r := range item.Stats.Rating.Ranks {
		if r.Name == "boardgame" {
			if n := r.Number(); n != nil {
				rank = *n
			}
			break
		}
	}
	thumb, image := item.Thumbnail, item.Image
	_, err := tx.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,year_published,image_url,thumbnail_url,
		min_players,max_players,min_playtime,max_playtime,min_age,bgg_rating,bgg_rank,is_expansion,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(bgg_id) DO UPDATE SET
			name=excluded.name,
			year_published=CASE WHEN excluded.year_published>0 THEN excluded.year_published ELSE games.year_published END,
			image_url=CASE WHEN excluded.image_url<>'' THEN excluded.image_url ELSE games.image_url END,
			thumbnail_url=CASE WHEN excluded.thumbnail_url<>'' THEN excluded.thumbnail_url ELSE games.thumbnail_url END,
			min_players=CASE WHEN excluded.min_players>0 THEN excluded.min_players ELSE games.min_players END,
			max_players=CASE WHEN excluded.max_players>0 THEN excluded.max_players ELSE games.max_players END,
			min_playtime=CASE WHEN excluded.min_playtime>0 THEN excluded.min_playtime ELSE games.min_playtime END,
			max_playtime=CASE WHEN excluded.max_playtime>0 THEN excluded.max_playtime ELSE games.max_playtime END,
			min_age=CASE WHEN excluded.min_age>0 THEN excluded.min_age ELSE games.min_age END,
			bgg_rating=CASE WHEN excluded.bgg_rating>0 THEN excluded.bgg_rating ELSE games.bgg_rating END,
			bgg_rank=CASE WHEN excluded.bgg_rank > 0 THEN excluded.bgg_rank ELSE games.bgg_rank END,
			is_expansion=CASE WHEN excluded.is_expansion=1 THEN 1 ELSE games.is_expansion END,
			updated_at=excluded.updated_at`,
		item.ObjectID, strings.TrimSpace(item.Name), strings.ToLower(strings.TrimSpace(item.Name)), item.YearPublished,
		image, thumb, item.Stats.MinPlayers, item.Stats.MaxPlayers, item.Stats.MinPlaytime, item.Stats.MaxPlaytime,
		item.Stats.MinAge, item.Stats.Rating.Average, rank, boolInt(strings.Contains(item.Subtype, "expansion")), now, now)
	if err != nil {
		return 0, fmt.Errorf("upsert game %d: %w", item.ObjectID, err)
	}
	var gameID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM games WHERE bgg_id=?`, item.ObjectID).Scan(&gameID); err != nil {
		return 0, err
	}
	rating := item.Stats.Rating.Personal()
	var ratingArg any
	if rating != nil {
		ratingArg = *rating
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_games(user_id,game_id,owned,wishlist,want_to_play,preordered,for_trade,
		user_rating,bgg_num_plays,added_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id,game_id) DO UPDATE SET
			owned=excluded.owned, wishlist=excluded.wishlist, want_to_play=excluded.want_to_play,
			preordered=excluded.preordered, for_trade=excluded.for_trade,
			user_rating=CASE WHEN user_games.user_rating IS NULL THEN excluded.user_rating ELSE user_games.user_rating END,
			bgg_num_plays=excluded.bgg_num_plays, updated_at=excluded.updated_at`,
		userID, gameID, boolInt(item.Status.IsOwned()), boolInt(item.Status.IsWishlisted()), boolInt(item.Status.IsWantToPlay()),
		boolInt(item.Status.IsPreordered()), boolInt(item.Status.IsForTrade()), ratingArg, item.NumPlays, now, now)
	if err != nil {
		return 0, fmt.Errorf("upsert shelf row %d: %w", item.ObjectID, err)
	}
	return gameID, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ClearRemovedOwnership unsets ownership for games no longer in the collection
// while retaining notes, plays, and the shelf row itself.
func ClearRemovedOwnership(ctx context.Context, tx *sql.Tx, userID string, keep []int64) error {
	now := db.Now()
	if len(keep) == 0 {
		_, err := tx.ExecContext(ctx, `UPDATE user_games SET owned=0, updated_at=? WHERE user_id=? AND owned=1`, now, userID)
		return err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(keep)), ",")
	args := []any{now, userID}
	for _, id := range keep {
		args = append(args, id)
	}
	_, err := tx.ExecContext(ctx, `UPDATE user_games SET owned=0, updated_at=? WHERE user_id=? AND owned=1 AND game_id NOT IN (`+placeholders+`)`, args...)
	return err
}

// UpsertThing writes full BGG metadata, junction rows, and refreshed FTS text.
func UpsertThing(ctx context.Context, tx *sql.Tx, t bgg.Thing) (int64, error) {
	now := db.Now()
	name := t.PrimaryName()
	rank := 0
	if n := t.Rank(); n != nil {
		rank = *n
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,year_published,description,image_url,thumbnail_url,
		min_players,max_players,min_playtime,max_playtime,min_age,weight,bgg_rating,bgg_rank,is_expansion,fetched_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(bgg_id) DO UPDATE SET
			name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE games.name END,
			sort_name=CASE WHEN excluded.name<>'' THEN excluded.sort_name ELSE games.sort_name END,
			year_published=excluded.year_published, description=excluded.description,
			image_url=CASE WHEN excluded.image_url<>'' THEN excluded.image_url ELSE games.image_url END,
			thumbnail_url=CASE WHEN excluded.thumbnail_url<>'' THEN excluded.thumbnail_url ELSE games.thumbnail_url END,
			min_players=excluded.min_players, max_players=excluded.max_players,
			min_playtime=excluded.min_playtime, max_playtime=excluded.max_playtime, min_age=excluded.min_age,
			weight=excluded.weight, bgg_rating=excluded.bgg_rating,
			bgg_rank=CASE WHEN excluded.bgg_rank>0 THEN excluded.bgg_rank ELSE games.bgg_rank END,
			is_expansion=CASE WHEN excluded.is_expansion=1 THEN 1 ELSE games.is_expansion END,
			fetched_at=excluded.fetched_at, updated_at=excluded.updated_at`,
		t.ID, name, strings.ToLower(name), t.YearPublished.Value, t.Description, t.Image, t.Thumbnail,
		t.MinPlayers.Value, t.MaxPlayers.Value, t.MinPlaytime.Value, t.MaxPlaytime.Value, t.MinAge.Value,
		t.Statistics.AverageWeight.Value, t.Statistics.Ratings.Average.Value, rank, boolInt(t.IsExpansion()),
		now, now, now)
	if err != nil {
		return 0, fmt.Errorf("upsert thing %d: %w", t.ID, err)
	}
	var gameID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM games WHERE bgg_id=?`, t.ID).Scan(&gameID); err != nil {
		return 0, err
	}
	if err := replaceLinks(ctx, tx, gameID, "game_categories", "categories", "category_id", t.LinksOfType("boardgamecategory")); err != nil {
		return 0, err
	}
	if err := replaceLinks(ctx, tx, gameID, "game_mechanics", "mechanics", "mechanic_id", t.LinksOfType("boardgamemechanic")); err != nil {
		return 0, err
	}
	if err := replaceLinks(ctx, tx, gameID, "game_designers", "designers", "designer_id", t.LinksOfType("boardgamedesigner")); err != nil {
		return 0, err
	}
	if err := replaceLinks(ctx, tx, gameID, "game_publishers", "publishers", "publisher_id", t.LinksOfType("boardgamepublisher")); err != nil {
		return 0, err
	}
	if err := RefreshFTS(ctx, tx, gameID); err != nil {
		return 0, err
	}
	return gameID, nil
}

func replaceLinks(ctx context.Context, tx *sql.Tx, gameID int64, junction, table, col string, names []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+junction+" WHERE game_id=?", gameID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+"(name) VALUES(?) ON CONFLICT(name) DO NOTHING", name); err != nil {
			return err
		}
		var id int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE name=? COLLATE NOCASE", name).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO "+junction+"(game_id,"+col+") VALUES(?,?)", gameID, id); err != nil {
			return err
		}
	}
	return nil
}

// RefreshFTS rebuilds one game's search document inside the caller's transaction.
func RefreshFTS(ctx context.Context, tx *sql.Tx, gameID int64) error {
	var name, description string
	if err := tx.QueryRowContext(ctx, `SELECT name,description FROM games WHERE id=?`, gameID).Scan(&name, &description); err != nil {
		return err
	}
	categories := joinNames(ctx, tx, `SELECT c.name FROM categories c JOIN game_categories gc ON gc.category_id=c.id WHERE gc.game_id=? ORDER BY c.name`, gameID)
	mechanics := joinNames(ctx, tx, `SELECT m.name FROM mechanics m JOIN game_mechanics gm ON gm.mechanic_id=m.id WHERE gm.game_id=? ORDER BY m.name`, gameID)
	designers := joinNames(ctx, tx, `SELECT d.name FROM designers d JOIN game_designers gd ON gd.designer_id=d.id WHERE gd.game_id=? ORDER BY d.name`, gameID)
	publishers := joinNames(ctx, tx, `SELECT p.name FROM publishers p JOIN game_publishers gp ON gp.publisher_id=p.id WHERE gp.game_id=? ORDER BY p.name`, gameID)
	if _, err := tx.ExecContext(ctx, `DELETE FROM games_fts WHERE rowid=?`, gameID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO games_fts(rowid,name,description,designers,publishers,categories,mechanics) VALUES(?,?,?,?,?,?,?)`,
		gameID, name, description, designers, publishers, categories, mechanics)
	return err
}

func joinNames(ctx context.Context, tx *sql.Tx, query string, gameID int64) string {
	rows, err := tx.QueryContext(ctx, query, gameID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

// RefreshAllFTS rebuilds the whole index; used after a large sync.
func RefreshAllFTS(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM games_fts`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM games`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := RefreshFTS(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

// EnsureStubGame creates a minimal catalogue row for a play whose game is not
// on the user's shelf.
func EnsureStubGame(ctx context.Context, tx *sql.Tx, bggID int, name string) (int64, error) {
	now := db.Now()
	_, err := tx.ExecContext(ctx, `INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(bgg_id) DO NOTHING`, bggID, name, strings.ToLower(name), now, now)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM games WHERE bgg_id=?`, bggID).Scan(&id)
	return id, err
}

// ParseIntString is a small helper for BGG attributes that may be blank.
func ParseIntString(v string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(v))
	return n
}

// NeedsEnrichment reports whether a game should be fetched from BGG.
func NeedsEnrichment(fetchedAt string, refreshDays int) bool {
	if fetchedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > time.Duration(refreshDays)*24*time.Hour
}
