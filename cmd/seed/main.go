// Command seed creates a believable local collection for development.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/bgg"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"
)

type seedGame struct {
	ID          int
	Name        string
	Year        int
	Min, Max    int
	MinTime     int
	MaxTime     int
	Age         int
	Weight      float64
	Rating      float64
	Rank        int
	Categories  []string
	Mechanics   []string
	Designers   []string
	Publishers  []string
	Description string
	Played      bool
	RatingSelf  string
	Notes       string
	Location    string
}

func main() {
	defaultDB := os.Getenv("DATABASE_PATH")
	if defaultDB == "" {
		defaultDB = "./data/boardgames.db"
	}
	dbPath := flag.String("db", defaultDB, "SQLite database path")
	flag.Parse()
	_ = os.Setenv("DATABASE_PATH", *dbPath)
	_ = os.Setenv("BGG_USER_AGENT", "TheShelfSeed/1.0 (dev@example.com)")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	database, err := db.Open(ctx, cfg.DatabasePath)
	if err != nil {
		panic(err)
	}
	defer database.Close()

	manager := auth.New(database, cfg)
	user, err := manager.UpsertUser(ctx, "owner@example.com", "Alex", "", true)
	if err != nil {
		panic(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE user_settings SET bgg_username='alexplays', bgg_username_verified_at=? WHERE user_id=?`, db.Now(), user.ID); err != nil {
		panic(err)
	}
	playRepo := plays.NewRepo(database)

	for i, g := range seedGames() {
		var gameID int64
		err := db.WithTx(ctx, database, func(tx *sql.Tx) error {
			t := bgg.Thing{ID: g.ID, Type: "boardgame", Image: fmt.Sprintf("https://cf.geekdo-images.com/seed-%d.jpg", g.ID), Thumbnail: fmt.Sprintf("https://cf.geekdo-images.com/seed-%d.jpg", g.ID), Description: g.Description}
			t.Names = []bgg.ThingName{{Type: "primary", Value: g.Name}}
			t.YearPublished.Value = g.Year
			t.MinPlayers.Value, t.MaxPlayers.Value = g.Min, g.Max
			t.MinPlaytime.Value, t.MaxPlaytime.Value = g.MinTime, g.MaxTime
			t.MinAge.Value = g.Age
			t.Statistics.AverageWeight.Value = g.Weight
			t.Statistics.Ratings.Average.Value = g.Rating
			t.Statistics.Ratings.Ranks = []bgg.Rank{{Type: "subtype", Name: "boardgame", Value: fmt.Sprintf("%d", g.Rank)}}
			for _, c := range g.Categories {
				t.Links = append(t.Links, bgg.Link{Type: "boardgamecategory", Value: c})
			}
			for _, m := range g.Mechanics {
				t.Links = append(t.Links, bgg.Link{Type: "boardgamemechanic", Value: m})
			}
			for _, d := range g.Designers {
				t.Links = append(t.Links, bgg.Link{Type: "boardgamedesigner", Value: d})
			}
			for _, p := range g.Publishers {
				t.Links = append(t.Links, bgg.Link{Type: "boardgamepublisher", Value: p})
			}
			var err error
			gameID, err = collection.UpsertThing(ctx, tx, t)
			if err != nil {
				return err
			}
			item := bgg.CollectionItem{ObjectID: g.ID, Name: g.Name, YearPublished: g.Year, Status: bgg.CollectionStatus{Own: "1"}}
			item.Image = t.Image
			item.Thumbnail = t.Thumbnail
			item.Stats.MinPlayers = g.Min
			item.Stats.MaxPlayers = g.Max
			item.Stats.MinPlaytime = g.MinTime
			item.Stats.MaxPlaytime = g.MaxTime
			item.Stats.MinAge = g.Age
			item.Stats.Rating.Average = g.Rating
			item.Stats.Rating.Value = g.RatingSelf
			if _, err := collection.UpsertCollectionItem(ctx, tx, user.ID, item); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
		acquired := time.Now().AddDate(0, 0, -((i*23)%420 + 12)).Format("2006-01-02")
		if _, err := database.ExecContext(ctx, `UPDATE user_games SET notes=?, location=?, acquisition_date=?, added_at=? WHERE user_id=? AND game_id=?`,
			g.Notes, g.Location, acquired, acquired+"T12:00:00Z", user.ID, gameID); err != nil {
			panic(err)
		}
		if g.Played {
			day := time.Now().AddDate(0, 0, -((i*9)%240 + 2)).Format("2006-01-02")
			_, err := playRepo.Create(ctx, user.ID, plays.Input{
				GameID: gameID, PlayedAt: day, Location: "The kitchen table", Duration: 60 + i*7, Quantity: 1,
				Notes: "A good one.", Players: []plays.Player{{Name: "Alex", Score: "42", IsWinner: i%2 == 0}, {Name: "Sam", Score: "38"}},
			}, "UTC")
			if err != nil {
				panic(err)
			}
		}
	}
	fmt.Printf("Seeded %d games for %s\n", len(seedGames()), user.Email)
}

func seedGames() []seedGame {
	return []seedGame{
		{101, "Wingspan", 2019, 1, 5, 40, 70, 10, 2.4, 8.1, 28, []string{"Animals", "Card Game"}, []string{"Hand Management", "Set Collection"}, []string{"Elizabeth Hargrave"}, []string{"Stonemaier Games"}, "A relaxing engine builder about attracting birds to your wildlife preserve. Each bird card combos with others, and every game tells a small story.", true, "9", "The blue heron card is banned in this house.", "Living room shelf"},
		{102, "Brass: Birmingham", 2018, 2, 4, 60, 120, 14, 3.9, 8.6, 1, []string{"Economic", "Industry / Manufacturing"}, []string{"Network Building", "Hand Management"}, []string{"Gavan Brown", "Matt Tolman", "Martin Wallace"}, []string{"Roxley"}, "A tight economic game of canals, coal, and iron in industrial Birmingham. Every decision is a small fight over tempo.", true, "8.5", "", "Game cupboard"},
		{103, "Ark Nova", 2021, 1, 4, 90, 150, 14, 3.7, 8.5, 3, []string{"Animals", "Environmental"}, []string{"Hand Management", "Tile Placement"}, []string{"Mathias Wigge"}, []string{"Feuerland Spiele"}, "Build a modern zoo and conservation project. The action-card puzzle rewards planning five moves ahead.", true, "9.5", "Our longest game so far. Worth it.", "Game cupboard"},
		{104, "Spirit Island", 2017, 1, 4, 90, 120, 13, 4.0, 8.4, 5, []string{"Fantasy", "Cooperative Game"}, []string{"Variable Player Powers", "Area Control"}, []string{"R. Eric Reuss"}, []string{"Greater Than Games"}, "Cooperative defence against colonisers, with enormous asymmetry. Hard, rewarding, and best with two.", true, "9", "", "Game cupboard"},
		{105, "Cascadia", 2021, 1, 4, 30, 45, 10, 1.8, 7.8, 51, []string{"Animals", "Puzzle"}, []string{"Tile Placement", "Pattern Building"}, []string{"Randy Flynn"}, []string{"Flatout Games"}, "A gentle tile-laying puzzle about Pacific Northwest habitats. Perfect with tea.", true, "8", "", "Coffee table stack"},
		{106, "Pandemic Legacy: Season 1", 2015, 2, 4, 60, 60, 12, 2.9, 7.6, 82, []string{"Medical", "Cooperative Game"}, []string{"Legacy Game", "Hand Management"}, []string{"Matt Leacock", "Rob Daviau"}, []string{"Z-Man Games"}, "The game that taught us to write on the board. A campaign with real heartbreak.", false, "", "", "Game cupboard"},
		{107, "Dune: Imperium", 2020, 1, 4, 60, 120, 14, 3.0, 8.2, 9, []string{"Science Fiction", "Political"}, []string{"Deck Building", "Worker Placement"}, []string{"Paul Dennen"}, []string{"Dire Wolf Digital"}, "Spice, intrigue, and a deck that never quite does what you want. Excellent tension.", true, "8", "", "Living room shelf"},
		{108, "The Crew: Mission Deep Sea", 2021, 2, 5, 20, 20, 10, 1.3, 7.7, 62, []string{"Card Game", "Cooperative Game"}, []string{"Trick-taking", "Communication Limits"}, []string{"Thomas Sing"}, []string{"KOSMOS"}, "A cooperative trick-taking campaign. Small box, enormous value.", true, "9", "", "Coffee table stack"},
		{109, "A Feast for Odin", 2016, 1, 4, 90, 120, 14, 3.8, 7.8, 45, []string{"Medieval", "Economic"}, []string{"Worker Placement", "Tile Placement"}, []string{"Uwe Rosenberg"}, []string{"Feuerland Spiele"}, "Vikings, raids, and the most satisfying polyomino puzzle in board games.", false, "", "", "Game cupboard"},
		{110, "Root", 2018, 2, 4, 60, 90, 10, 3.3, 7.6, 34, []string{"Animals", "Wargame"}, []string{"Asymmetric Powers", "Area Control"}, []string{"Cole Wehrle"}, []string{"Leder Games"}, "A woodland war where every faction plays a different game. Beautiful and sharp.", false, "", "", "Game cupboard"},
		{111, "Patchwork", 2014, 2, 2, 15, 30, 8, 1.6, 7.5, 96, []string{"Abstract Strategy", "Puzzle"}, []string{"Tile Placement", "Time Track"}, []string{"Uwe Rosenberg"}, []string{"Lookout Games"}, "Two-player quilting with a brutal little economy. Ten minutes, endless rematches.", true, "8", "", "Coffee table stack"},
		{112, "Sleeping Gods", 2021, 1, 4, 60, 120, 13, 3.4, 7.7, 71, []string{"Adventure", "Exploration"}, []string{"Storytelling", "Worker Placement"}, []string{"Ryan Laukat"}, []string{"Red Raven Games"}, "A campaign of exploration aboard the Manticore. The atlas is the star.", false, "", "", "Game cupboard"},
	}
}

// ObjType keeps the seed tolerant of BGG collection object typing.
func (g seedGame) ObjTypeString() string { return "thing" }
