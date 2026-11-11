package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"boardgames/internal/db"
)

func (f *playFormFixture) submitPlay(t *testing.T, path, location string) *httptest.ResponseRecorder {
	t.Helper()
	values := url.Values{
		"game_id":   {strconv.FormatInt(f.gameID, 10)},
		"played_at": {"2026-03-04"},
		"quantity":  {"1"},
		"location":  {location},
	}
	return f.post(t, path, values)
}

func (f *playFormFixture) latestPlay(t *testing.T) (id, location string, locationID any) {
	t.Helper()
	var locID any
	if err := f.db.QueryRow(`SELECT id, location, location_id FROM plays WHERE user_id=? ORDER BY rowid DESC LIMIT 1`, f.owner.ID).Scan(&id, &location, &locID); err != nil {
		t.Fatal(err)
	}
	return id, location, locID
}

func TestPlayWritesResolveVenues(t *testing.T) {
	f := setupPlayForm(t)

	// Two spellings of one venue collapse to a single row and both plays link
	// to it, with the first spelling kept as the canonical display name.
	if rr := f.submitPlay(t, "/plays", "The Crown"); rr.Code != http.StatusSeeOther {
		t.Fatalf("create play: %d %s", rr.Code, rr.Body)
	}
	id1, location1, link1 := f.latestPlay(t)
	if location1 != "The Crown" || link1 == nil {
		t.Fatalf("first play not resolved: %q %v", location1, link1)
	}

	if rr := f.submitPlay(t, "/plays", "the crown"); rr.Code != http.StatusSeeOther {
		t.Fatalf("create second play: %d %s", rr.Code, rr.Body)
	}
	_, location2, link2 := f.latestPlay(t)
	if location2 != "The Crown" {
		t.Fatalf("canonical spelling not stored, got %q", location2)
	}
	if link1 != link2 {
		t.Fatalf("spellings linked to different venues: %v != %v", link1, link2)
	}
	var venues int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id=?`, f.owner.ID).Scan(&venues); err != nil {
		t.Fatal(err)
	}
	if venues != 1 {
		t.Fatalf("expected one venue row, got %d", venues)
	}

	// Empty input stays unlinked.
	if rr := f.submitPlay(t, "/plays", "   "); rr.Code != http.StatusSeeOther {
		t.Fatalf("create empty play: %d %s", rr.Code, rr.Body)
	}
	_, location3, link3 := f.latestPlay(t)
	if location3 != "" || link3 != nil {
		t.Fatalf("empty location should stay unlinked, got %q %v", location3, link3)
	}

	// Editing re-resolves and moves the link.
	if rr := f.submitPlay(t, "/plays/"+id1, "The Tavern"); rr.Code != http.StatusSeeOther {
		t.Fatalf("edit play: %d %s", rr.Code, rr.Body)
	}
	var locationID int64
	var name string
	if err := f.db.QueryRow(`SELECT l.id, l.name FROM plays p JOIN locations l ON l.id=p.location_id WHERE p.id=?`, id1).Scan(&locationID, &name); err != nil {
		t.Fatal(err)
	}
	if name != "The Tavern" || locationID == 0 {
		t.Fatalf("edit did not re-resolve: %d %q", locationID, name)
	}

	// Clearing the location on edit unlinks it.
	if rr := f.submitPlay(t, "/plays/"+id1, ""); rr.Code != http.StatusSeeOther {
		t.Fatalf("clear location: %d %s", rr.Code, rr.Body)
	}
	var link any
	if err := f.db.QueryRow(`SELECT location_id FROM plays WHERE id=?`, id1).Scan(&link); err != nil {
		t.Fatal(err)
	}
	if link != nil {
		t.Fatalf("cleared location should unlink, got %v", link)
	}
}

func TestStatsPageShowsVenuesAndBackfills(t *testing.T) {
	f := setupPlayForm(t)
	now := db.Now()
	res, err := f.db.Exec(`INSERT INTO games(bgg_id,name,sort_name,created_at,updated_at) VALUES(14,'Wingspan','wingspan',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	secondGame, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		id, location, playedAt string
		game                   int64
	}{
		{"legacy1", "The Crown", "2026-01-01", f.gameID},
		{"legacy2", "The Crown", "2026-01-02", f.gameID},
		{"legacy3", "the crown", "2026-01-03", secondGame},
	} {
		if _, err := f.db.Exec(`INSERT INTO plays(id,user_id,game_id,played_at,location,quantity,created_at,updated_at)
			VALUES(?,?,?,?,?,1,?,?)`, p.id, f.owner.ID, p.game, p.playedAt, p.location, now, now); err != nil {
			t.Fatal(err)
		}
	}

	rr := f.get(t, "/stats")
	if rr.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rr.Code, rr.Body)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Where you play",
		"Most of your plays happen at The Crown (3 plays across 2 games).",
		"3 Jan 2026",
	} {
		if !strings.Contains(body, want) {
			i := strings.Index(body, "Most of your plays")
			excerpt := body
			if i >= 0 && i+160 < len(body) {
				excerpt = body[i : i+160]
			}
			t.Errorf("stats page missing %q (excerpt %q)", want, excerpt)
		}
	}

	var venues, linked int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id=?`, f.owner.ID).Scan(&venues); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND location_id IS NOT NULL`, f.owner.ID).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if venues != 1 || linked != 3 {
		t.Fatalf("expected one venue and three linked plays, got %d %d", venues, linked)
	}

	// A second render is a no-op: same rows, nothing relinked.
	rr = f.get(t, "/stats")
	if rr.Code != http.StatusOK {
		t.Fatalf("second stats: %d %s", rr.Code, rr.Body)
	}
	var venuesAfter int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM locations WHERE user_id=?`, f.owner.ID).Scan(&venuesAfter); err != nil {
		t.Fatal(err)
	}
	if venuesAfter != venues {
		t.Fatalf("second backfill created venues: %d -> %d", venues, venuesAfter)
	}
}

func TestStatsPageOmitsVenuesWhenEmpty(t *testing.T) {
	f := setupPlayForm(t)
	rr := f.get(t, "/stats")
	if rr.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), "Where you play") {
		t.Fatal("empty stats page should not render the venue section")
	}
}

func TestPlayFormListsKnownVenues(t *testing.T) {
	f := setupPlayForm(t)
	if rr := f.submitPlay(t, "/plays", "The Crown"); rr.Code != http.StatusSeeOther {
		t.Fatalf("create play: %d %s", rr.Code, rr.Body)
	}
	rr := f.get(t, "/plays/new")
	if rr.Code != http.StatusOK {
		t.Fatalf("play form: %d %s", rr.Code, rr.Body)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `list="known-venues"`) || !strings.Contains(body, `<option value="The Crown">`) {
		t.Fatalf("play form missing venue datalist: %s", body)
	}
	if !strings.Contains(body, `name="client_id"`) {
		t.Fatal("client_id hidden field was disturbed")
	}
}
