package demo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"boardgames/internal/bgg"
	"boardgames/internal/collection"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

type sampleGame struct {
	bggID       int
	name        string
	year        int
	description string
	minPlayers  int
	maxPlayers  int
	minPlaytime int
	maxPlaytime int
	minAge      int
	weight      float64
	rating      float64
	rank        int
	notes       string
	location    string
	acquired    string
	userRating  float64
	category    string
	mechanic    string
}

var sampleGames = []sampleGame{
	{13, "Catan", 1995, "Trade, build, and settle the island of Catan. A modern classic of resource management.", 3, 4, 60, 120, 10, 2.3, 7.1, 180, "Corner box is a little scuffed after years of play.", "Living room shelf", "2019-03-14", 8, "Economic", "Dice Rolling"},
	{266192, "Wingspan", 2019, "A competitive engine builder about attracting birds to your wildlife preserve.", 1, 5, 40, 70, 10, 2.4, 8.1, 25, "The egg tokens are a house favourite.", "Living room shelf", "2020-11-02", 9, "Animals", "Engine Building"},
	{9209, "Ticket to Ride", 2004, "Collect train cards and claim routes across a map while completing destination tickets.", 2, 5, 30, 60, 8, 1.9, 7.4, 120, "Travel-sized fun; easy to teach to guests.", "Hall cupboard", "2018-07-21", 7.5, "Trains", "Set Collection"},
	{30549, "Pandemic", 2008, "Work together to stop four diseases from spreading across the world.", 2, 4, 45, 45, 8, 2.4, 7.6, 140, "Co-op standby for rainy afternoons.", "Hall cupboard", "2017-01-08", 8, "Medical", "Cooperative Game"},
	{230802, "Azul", 2017, "Draft colourful tiles to decorate the royal palace of Evora.", 2, 4, 30, 45, 8, 1.8, 7.8, 60, "Beautiful components and a sharp little puzzle.", "Living room shelf", "2021-05-16", 8.5, "Abstract Strategy", "Tile Placement"},
	{174430, "Gloomhaven", 2017, "A sprawling campaign of tactical combat and persistent character progression.", 1, 4, 60, 120, 14, 3.8, 8.6, 15, "The big box we keep promising to finish.", "Shelf of opportunity", "2022-09-30", 9, "Adventure", "Campaign Game"},
}

// ensureSampleGame reuses an existing shared catalogue row untouched. Only a
// missing game is created through the BGG upsert helpers.
func ensureSampleGame(ctx context.Context, tx *sql.Tx, userID string, g sampleGame) (int64, error) {
	var existingID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM games WHERE bgg_id=?`, g.bggID).Scan(&existingID)
	switch {
	case err == nil:
		if err := ensureShelfRow(ctx, tx, userID, existingID); err != nil {
			return 0, err
		}
		return existingID, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, err
	}
	thing := bgg.Thing{
		ID:            g.bggID,
		Type:          "boardgame",
		Names:         []bgg.ThingName{{Type: "primary", Value: g.name}},
		Description:   g.description,
		YearPublished: bgg.IntValue{Value: g.year},
		MinPlayers:    bgg.IntValue{Value: g.minPlayers},
		MaxPlayers:    bgg.IntValue{Value: g.maxPlayers},
		MinPlaytime:   bgg.IntValue{Value: g.minPlaytime},
		MaxPlaytime:   bgg.IntValue{Value: g.maxPlaytime},
		MinAge:        bgg.IntValue{Value: g.minAge},
		Statistics: bgg.ThingStats{
			AverageWeight: bgg.FloatValue{Value: g.weight},
		},
		Links: []bgg.Link{
			{Type: "boardgamecategory", Value: g.category},
			{Type: "boardgamemechanic", Value: g.mechanic},
		},
	}
	if _, err := collection.UpsertThing(ctx, tx, thing); err != nil {
		return 0, err
	}
	item := bgg.CollectionItem{
		ObjectType:    "thing",
		ObjectID:      g.bggID,
		Subtype:       "boardgame",
		Name:          g.name,
		YearPublished: g.year,
		Stats: bgg.CollectionStats{
			MinPlayers: g.minPlayers, MaxPlayers: g.maxPlayers,
			MinPlaytime: g.minPlaytime, MaxPlaytime: g.maxPlaytime, MinAge: g.minAge,
			Rating: bgg.BGGRating{Average: g.rating, Ranks: []bgg.Rank{{Name: "boardgame", Value: fmt.Sprint(g.rank)}}},
		},
		Status: bgg.CollectionStatus{Own: "1"},
	}
	return collection.UpsertCollectionItem(ctx, tx, userID, item)
}

// ensureShelfRow adds the demo's shelf link without modifying shared game data.
func ensureShelfRow(ctx context.Context, tx *sql.Tx, userID string, gameID int64) error {
	now := db.Now()
	_, err := tx.ExecContext(ctx, `INSERT INTO user_games(user_id,game_id,owned,added_at,updated_at)
		VALUES(?,?,1,?,?) ON CONFLICT(user_id,game_id) DO NOTHING`, userID, gameID, now, now)
	return err
}

// seed adds a small, generic sample shelf and play log once. It is a no-op
// when the demo user already has shelf rows or plays.
func (s *Service) seed(ctx context.Context, userID string) error {
	var shelfCount int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_games WHERE user_id=?`, userID).Scan(&shelfCount); err != nil {
		return err
	}
	gameIDs := map[int]int64{}
	if shelfCount == 0 {
		err := db.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
			for _, g := range sampleGames {
				gameID, err := ensureSampleGame(ctx, tx, userID, g)
				if err != nil {
					return err
				}
				gameIDs[g.bggID] = gameID
				if _, err := tx.ExecContext(ctx, `UPDATE user_games SET notes=?,location=?,acquisition_date=?,user_rating=?,updated_at=?
					WHERE user_id=? AND game_id=?`, g.notes, g.location, g.acquired, g.userRating, db.Now(), userID, gameID); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	var playCount int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=?`, userID).Scan(&playCount); err != nil {
		return err
	}
	if playCount > 0 {
		return nil
	}

	if len(gameIDs) == 0 {
		// Shelf rows existed before this run (for example after a partial seed).
		rows, err := s.DB.QueryContext(ctx, `SELECT g.bgg_id,ug.game_id FROM user_games ug JOIN games g ON g.id=ug.game_id
			WHERE ug.user_id=? ORDER BY ug.game_id LIMIT 4`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var bggID int
			var gameID int64
			if err := rows.Scan(&bggID, &gameID); err != nil {
				rows.Close()
				return err
			}
			gameIDs[bggID] = gameID
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}

	playSpecs := []struct {
		bggID    int
		daysAgo  int
		duration int
		notes    string
		alex     string
		sam      string
		alexWins bool
	}{
		{13, 2, 75, "A close game decided by the longest road.", "10", "9", true},
		{266192, 9, 55, "Sam built a strong wetland engine.", "72", "81", false},
		{9209, 16, 40, "Quick evening game with tea.", "118", "102", true},
		{230802, 23, 35, "First outing for the new tile bag.", "64", "58", true},
	}
	repo := plays.NewRepo(s.DB)
	now := time.Now().UTC()
	for i, spec := range playSpecs {
		gameID, ok := gameIDs[spec.bggID]
		if !ok {
			continue
		}
		players := []plays.Player{{Name: "Alex", Score: spec.alex, IsWinner: spec.alexWins}}
		if !spec.alexWins {
			players[0].IsWinner = false
		}
		players = append(players, plays.Player{Name: "Sam", Score: spec.sam, IsWinner: !spec.alexWins})
		in := plays.Input{
			GameID:   gameID,
			PlayedAt: now.AddDate(0, 0, -spec.daysAgo).Format("2006-01-02"),
			Location: "Home",
			Duration: spec.duration,
			Quantity: 1,
			Notes:    spec.notes,
			Players:  players,
		}
		if _, err := repo.Create(ctx, userID, in, "UTC"); err != nil {
			return fmt.Errorf("create sample play %d: %w", i+1, err)
		}
	}
	return nil
}
