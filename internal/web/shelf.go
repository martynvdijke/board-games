package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/plays"
)

func (h *Handler) parseFilters(r *http.Request) collection.Filters {
	q := r.URL.Query()
	f := collection.Filters{
		Query:       strings.TrimSpace(q.Get("q")),
		Players:     parseInt(q.Get("players"), 0),
		MaxPlaytime: parseInt(q.Get("max_time"), 0),
		MinWeight:   parseFloat(q.Get("min_weight")),
		MaxWeight:   parseFloat(q.Get("max_weight")),
		Category:    strings.TrimSpace(q.Get("category")),
		Mechanic:    strings.TrimSpace(q.Get("mechanic")),
		Status:      q.Get("status"),
		Sort:        q.Get("sort"),
		Page:        parseInt(q.Get("page"), 1),
		Density:     q.Get("view"),
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Players > 20 {
		f.Players = 20
	}
	if f.MaxPlaytime > 600 {
		f.MaxPlaytime = 600
	}
	switch f.Status {
	case "owned", "wishlist", "want_to_play", "preordered", "for_trade", "favorite", "unplayed":
	default:
		f.Status = ""
	}
	switch f.Sort {
	case "name", "year", "rating", "bgg_rating", "weight", "weight_desc", "added", "plays":
	default:
		f.Sort = "name"
	}
	if f.Density != "list" && f.Density != "grid" {
		f.Density = ""
		if c, err := r.Cookie("shelf_view"); err == nil && c.Value == "list" {
			f.Density = "list"
		} else {
			f.Density = "grid"
		}
	}
	return f
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	f := h.parseFilters(r)
	if v := r.URL.Query().Get("view"); v == "grid" || v == "list" {
		http.SetCookie(w, &http.Cookie{Name: "shelf_view", Value: v, Path: "/", MaxAge: 31536000, SameSite: http.SameSiteLaxMode, HttpOnly: true, Secure: h.Config.SecureCookies()})
	}
	games, total, err := h.Games.List(ctx, user.ID, f)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load the shelf", err)
		return
	}
	facets, err := h.Games.Facets(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load the filters", err)
		return
	}
	stats, err := h.Games.Stats(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your stats", err)
		return
	}
	data := &OverviewData{Filters: f, Games: games, Total: total, Facets: facets, Stats: stats, View: f.Density,
		Greeting: greeting(time.Now()), FirstName: firstName(user.Name)}
	if payload, err := h.TRMNL.Build(ctx, user.ID); err == nil {
		data.Pick = payload.Pick
	}
	size := pageSize(f)
	data.PageCount = int(math.Ceil(float64(total) / float64(size)))
	if data.PageCount < 1 {
		data.PageCount = 1
	}
	data.Pages = pageWindow(f.Page, data.PageCount)
	p := h.page(r, "Your shelf", "shelf", "")
	p.Data = data
	h.render(w, r, "overview.html", p)
}

func (h *Handler) gamesPartial(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	f := h.parseFilters(r)
	games, total, err := h.Games.List(ctx, user.ID, f)
	if err != nil {
		http.Error(w, "could not load games", http.StatusInternalServerError)
		return
	}
	stats, _ := h.Games.Stats(ctx, user.ID)
	pageCount := int(math.Ceil(float64(total) / float64(pageSize(f))))
	if pageCount < 1 {
		pageCount = 1
	}
	data := &OverviewData{Filters: f, Games: games, Total: total, Stats: stats, View: f.Density,
		PageCount: pageCount, Pages: pageWindow(f.Page, pageCount)}
	h.renderPartial(w, "shelf_results", data)
}

func (h *Handler) gameDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "That game isn't here", nil)
		return
	}
	game, err := h.Games.Get(ctx, user.ID, id)
	if err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			h.errorPage(w, r, http.StatusNotFound, "That game isn't on this shelf", nil)
			return
		}
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't open that game", err)
		return
	}
	p := h.page(r, game.Name, "shelf", "")
	p.Data = &GameData{Game: game, PlayToday: time.Now().Format("2006-01-02")}
	h.render(w, r, "game.html", p)
}

func (h *Handler) updateCollection(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "Game not found", nil)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.errorPage(w, r, http.StatusBadRequest, "That form couldn't be read", err)
		return
	}
	yes := func(key string) *bool {
		v := r.PostForm.Has(key)
		return &v
	}
	patch := collection.CollectionPatch{
		Owned: yes("owned"), Wishlist: yes("wishlist"), WantToPlay: yes("want_to_play"),
		Preordered: yes("preordered"), ForTrade: yes("for_trade"), Favorite: yes("favorite"),
	}
	rating := r.PostForm.Get("user_rating")
	if rating == "" || rating == "none" {
		patch.ClearRating = true
	} else if v, err := strconv.ParseFloat(rating, 64); err == nil {
		patch.UserRating = &v
	} else {
		h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10), "Rating must be a number", "error")
		return
	}
	notes := r.PostForm.Get("notes")
	acquired := r.PostForm.Get("acquisition_date")
	location := r.PostForm.Get("location")
	condition := r.PostForm.Get("condition")
	patch.Notes, patch.AcquisitionDate, patch.Location, patch.Condition = &notes, &acquired, &location, &condition
	price := strings.TrimSpace(r.PostForm.Get("price_paid"))
	if price == "" {
		patch.ClearPrice = true
	} else if v, err := strconv.ParseFloat(price, 64); err == nil && v >= 0 {
		cents := int(math.Round(v * 100))
		patch.PricePaid = &cents
	} else {
		h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10), "Price must be a positive number", "error")
		return
	}
	if err := h.Games.Apply(ctx, user.ID, id, patch); err != nil {
		h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10), err.Error(), "error")
		return
	}
	h.flashRedirect(w, r, "/games/"+strconv.FormatInt(id, 10), "Shelf details saved", "success")
}

func (h *Handler) newPlayPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	games, err := h.Plays.ShelfGames(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your games", err)
		return
	}
	selected := int64(parseInt(r.URL.Query().Get("game"), 0))
	data := &PlayFormData{Games: games, Today: time.Now().Format("2006-01-02"), SelectedGameID: selected,
		Attempt: playAttempt{PlayedAt: time.Now().Format("2006-01-02"), Quantity: 1, Players: blankPlayers(2)}}
	p := h.page(r, "Log a play", "plays", "")
	p.Data = data
	h.render(w, r, "play_form.html", p)
}

func (h *Handler) createPlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	attempt, in := parsePlayForm(r)
	settings := h.settingsFor(r, user.ID)
	if _, err := h.Plays.Create(ctx, user.ID, in, settings.Timezone); err != nil {
		h.renderPlayForm(w, r, attempt, "new", err)
		return
	}
	h.flashRedirect(w, r, "/plays", "Play logged", "success")
}

func (h *Handler) editPlayPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	play, err := h.Plays.Get(ctx, user.ID, r.PathValue("id"))
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "That play isn't here", err)
		return
	}
	if play.Origin != "local" {
		h.flashRedirect(w, r, "/plays", "Imported BGG plays are read-only", "error")
		return
	}
	games, err := h.Plays.ShelfGames(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your games", err)
		return
	}
	data := &PlayFormData{Play: play, Games: games, Today: time.Now().Format("2006-01-02"), Editing: true, SelectedGameID: play.GameID,
		Attempt: playAttempt{GameID: play.GameID, PlayedAt: play.PlayedAt, Location: play.Location, Duration: play.Duration, Quantity: play.Quantity, Notes: play.Notes, Players: play.Players}}
	p := h.page(r, "Edit play", "plays", "")
	p.Data = data
	h.render(w, r, "play_form.html", p)
}

func (h *Handler) updatePlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	attempt, in := parsePlayForm(r)
	settings := h.settingsFor(r, user.ID)
	if _, err := h.Plays.Update(ctx, user.ID, r.PathValue("id"), in, settings.Timezone); err != nil {
		h.renderPlayForm(w, r, attempt, "edit", err)
		return
	}
	h.flashRedirect(w, r, "/plays", "Play updated", "success")
}

func (h *Handler) deletePlay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	if err := h.Plays.Delete(ctx, user.ID, r.PathValue("id")); err != nil {
		h.flashRedirect(w, r, "/plays", "That play couldn't be deleted", "error")
		return
	}
	h.flashRedirect(w, r, "/plays", "Play deleted", "success")
}

func (h *Handler) renderPlayForm(w http.ResponseWriter, r *http.Request, attempt playAttempt, mode string, renderErr error) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	games, _ := h.Plays.ShelfGames(ctx, user.ID)
	data := &PlayFormData{Games: games, Today: time.Now().Format("2006-01-02"), Editing: mode == "edit", SelectedGameID: attempt.GameID, Attempt: attempt}
	if mode == "edit" {
		if play, err := h.Plays.Get(ctx, user.ID, r.PathValue("id")); err == nil {
			data.Play = play
		}
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	p := h.page(r, "Log a play", "plays", "")
	p.Error = renderErr.Error()
	p.Data = data
	h.render(w, r, "play_form.html", p)
}

func parsePlayForm(r *http.Request) (playAttempt, plays.Input) {
	_ = r.ParseForm()
	gameID, _ := strconv.ParseInt(r.PostForm.Get("game_id"), 10, 64)
	quantity := parseInt(r.PostForm.Get("quantity"), 1)
	duration := parseInt(r.PostForm.Get("duration"), 0)
	attempt := playAttempt{GameID: gameID, PlayedAt: r.PostForm.Get("played_at"), Location: strings.TrimSpace(r.PostForm.Get("location")),
		Duration: duration, Quantity: quantity, Notes: strings.TrimSpace(r.PostForm.Get("notes"))}
	names := r.PostForm["player_name"]
	scores := r.PostForm["player_score"]
	winner := map[string]bool{}
	for _, v := range r.PostForm["player_winner"] {
		winner[v] = true
	}
	first := map[string]bool{}
	for _, v := range r.PostForm["player_first"] {
		first[v] = true
	}
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		score := ""
		if i < len(scores) {
			score = strings.TrimSpace(scores[i])
		}
		key := strconv.Itoa(i)
		attempt.Players = append(attempt.Players, plays.Player{Name: name, Score: score, IsWinner: winner[key], IsFirst: first[key]})
	}
	return attempt, plays.Input{GameID: gameID, PlayedAt: attempt.PlayedAt, Location: attempt.Location, Duration: duration,
		Quantity: quantity, Notes: attempt.Notes, Players: attempt.Players}
}

func blankPlayers(n int) []plays.Player {
	out := make([]plays.Player, n)
	return out
}

func (h *Handler) playsPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	year := parseInt(r.URL.Query().Get("year"), 0)
	gameID := int64(parseInt(r.URL.Query().Get("game"), 0))
	page := parseInt(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	const limit = 50
	list, total, err := h.Plays.List(ctx, user.ID, plays.ListOptions{GameID: gameID, Year: year, Limit: limit, Offset: (page - 1) * limit})
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your plays", err)
		return
	}
	years, _ := h.Plays.PlayYears(ctx, user.ID)
	games, _ := h.Plays.PlayedGames(ctx, user.ID)
	pageCount := int(math.Ceil(float64(total) / limit))
	if pageCount < 1 {
		pageCount = 1
	}
	base := url.Values{}
	if year > 0 {
		base.Set("year", strconv.Itoa(year))
	}
	if gameID > 0 {
		base.Set("game", strconv.FormatInt(gameID, 10))
	}
	withPage := func(n int) string {
		v := url.Values{}
		for k, vv := range base {
			v[k] = vv
		}
		if n > 1 {
			v.Set("page", strconv.Itoa(n))
		}
		if enc := v.Encode(); enc != "" {
			return "/plays?" + enc
		}
		return "/plays"
	}
	data := &PlaysData{Groups: groupPlays(list), Total: total, Year: year, GameID: gameID, Years: years, Games: games,
		Page: page, PageCount: pageCount}
	if page > 1 {
		data.PrevURL = withPage(page - 1)
	}
	if page < pageCount {
		data.NextURL = withPage(page + 1)
	}
	p := h.page(r, "Play log", "plays", "")
	p.Data = data
	h.render(w, r, "plays.html", p)
}

func (h *Handler) statsPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	year := parseInt(r.URL.Query().Get("year"), time.Now().Year())
	stats, err := h.Plays.Stats(ctx, user.ID, year)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't build your stats", err)
		return
	}
	shelf, err := h.Games.Stats(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your shelf stats", err)
		return
	}
	years, _ := h.Plays.PlayYears(ctx, user.ID)
	chartMax, topMax := 1, 1
	for _, m := range stats.Monthly {
		if m.Count > chartMax {
			chartMax = m.Count
		}
	}
	for _, g := range stats.TopGames {
		if g.Count > topMax {
			topMax = g.Count
		}
	}
	p := h.page(r, "Stats", "stats", "")
	p.Data = &StatsData{Stats: stats, Shelf: shelf, Years: years, ChartMax: chartMax, TopMax: topMax}
	h.render(w, r, "stats.html", p)
}

func (h *Handler) errorPage(w http.ResponseWriter, r *http.Request, status int, message string, err error) {
	if err != nil {
		// The message is user-facing; details stay in the structured log.
		_ = err
	}
	w.WriteHeader(status)
	p := h.page(r, fmt.Sprintf("%d", status), "", "")
	p.Error = message
	p.Data = map[string]any{"Status": status, "Message": message}
	h.render(w, r, "error.html", p)
}

func greeting(t time.Time) string {
	switch {
	case t.Hour() < 12:
		return "Good morning"
	case t.Hour() < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}

func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return name
}

func pageWindow(current, total int) []int {
	if total <= 0 {
		return nil
	}
	start, end := current-2, current+2
	if start < 1 {
		end += 1 - start
		start = 1
	}
	if end > total {
		start -= end - total
		end = total
		if start < 1 {
			start = 1
		}
	}
	var out []int
	for i := start; i <= end; i++ {
		out = append(out, i)
	}
	return out
}
