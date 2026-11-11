package web

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/db"
)

type setupWebFixture struct {
	handler *Handler
	db      *sql.DB
	owner   *auth.User
	other   *auth.User
	gameID  int64
	csrf    string
}

func newSetupWebFixture(t *testing.T) *setupWebFixture {
	t.Helper()
	ctx := context.Background()
	cfg := &config.Config{AppName: "The Shelf", BaseURL: "http://localhost:6270", Timezone: "UTC"}
	database, err := db.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	manager := auth.New(database, cfg)
	owner, err := manager.UpsertUser(ctx, "owner@example.com", "Owner", "", true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.UpsertUser(ctx, "other@example.com", "Other", "", true)
	if err != nil {
		t.Fatal(err)
	}
	now := db.Now()
	res, err := database.Exec(`INSERT INTO games (bgg_id,name,created_at,updated_at) VALUES (?,?,?,?)`, 266192, "Wingspan", now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO user_games (user_id,game_id,owned,added_at,updated_at) VALUES (?,?,1,?,?)`, owner.ID, gameID, now, now); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Config: cfg, Auth: manager, DB: database, Games: collection.NewRepo(database)}
	return &setupWebFixture{handler: h, db: database, owner: owner, other: other, gameID: gameID, csrf: "test-csrf"}
}

// newRequest builds a request carrying the authenticated user, CSRF token and
// the game path value the handlers rely on.
func (f *setupWebFixture) newRequest(t *testing.T, method, target, contentType string, body []byte, user *auth.User) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.SetPathValue("id", strconv.FormatInt(f.gameID, 10))
	return req.WithContext(auth.WithAuth(req.Context(), &auth.AuthContext{User: user, CSRFToken: f.csrf, Via: "cookie"}))
}

func (f *setupWebFixture) move(t *testing.T, op, imageID string, user *auth.User) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"op": {op}, "image_id": {imageID}, "csrf_token": {f.csrf}}
	target := "/games/" + strconv.FormatInt(f.gameID, 10) + "/setup"
	req := f.newRequest(t, http.MethodPost, target, "application/x-www-form-urlencoded", []byte(form.Encode()), user)
	rec := httptest.NewRecorder()
	f.handler.updateSetup(rec, req)
	return rec
}

func (f *setupWebFixture) addImage(t *testing.T, id, caption string) {
	t.Helper()
	if err := f.handler.Games.AddSetupImage(context.Background(), f.owner.ID, f.gameID, id, id+".jpg", "image/jpeg", caption); err != nil {
		t.Fatal(err)
	}
}

func (f *setupWebFixture) imageIDs(t *testing.T) []string {
	t.Helper()
	detail, err := f.handler.Games.Get(context.Background(), f.owner.ID, f.gameID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(detail.SetupImages))
	for _, img := range detail.SetupImages {
		ids = append(ids, img.ID)
	}
	return ids
}

func TestUploadSetupImageRespectsNewLimit(t *testing.T) {
	f := newSetupWebFixture(t)
	ctx := context.Background()
	for i := 0; i < collection.MaxSetupImages; i++ {
		f.addImage(t, "img-"+strconv.Itoa(i), "")
	}
	count, err := f.handler.Games.CountSetupImages(ctx, f.owner.ID, f.gameID)
	if err != nil || count != collection.MaxSetupImages {
		t.Fatalf("count = %d, err = %v", count, err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("image", "step.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\n")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	target := "/games/" + strconv.FormatInt(f.gameID, 10) + "/setup/images"
	req := f.newRequest(t, http.MethodPost, target, mw.FormDataContentType(), body.Bytes(), f.owner)
	rec := httptest.NewRecorder()
	f.handler.uploadSetupImage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	location := rec.Header().Get("Location")
	want := "up to " + strconv.Itoa(collection.MaxSetupImages) + " photos"
	if !strings.Contains(location, "flash=") || !strings.Contains(location, url.QueryEscape(want)) {
		t.Fatalf("redirect %q does not report the %d-photo limit", location, collection.MaxSetupImages)
	}
	count, err = f.handler.Games.CountSetupImages(ctx, f.owner.ID, f.gameID)
	if err != nil || count != collection.MaxSetupImages {
		t.Fatalf("count after rejected upload = %d, err = %v", count, err)
	}
}

func TestSetupImageReorderPersistsContiguousPositions(t *testing.T) {
	f := newSetupWebFixture(t)
	f.addImage(t, "img-1", "Alpha")
	f.addImage(t, "img-2", "Beta")
	f.addImage(t, "img-3", "Gamma")

	rec := f.move(t, "move_up", "img-3", f.owner)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("move up status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := f.imageIDs(t), []string{"img-1", "img-3", "img-2"}; !equalStrings(got, want) {
		t.Fatalf("order after move up = %v, want %v", got, want)
	}
	assertPositions(t, f.db, f.owner.ID, f.gameID)

	rec = f.move(t, "move_down", "img-3", f.owner)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("move down status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := f.imageIDs(t), []string{"img-1", "img-2", "img-3"}; !equalStrings(got, want) {
		t.Fatalf("order after move down = %v, want %v", got, want)
	}
	assertPositions(t, f.db, f.owner.ID, f.gameID)
}

func TestSetupImageReorderRejectsOtherUser(t *testing.T) {
	f := newSetupWebFixture(t)
	f.addImage(t, "img-1", "Alpha")
	f.addImage(t, "img-2", "Beta")

	rec := f.move(t, "move_up", "img-2", f.other)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user move status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got, want := f.imageIDs(t), []string{"img-1", "img-2"}; !equalStrings(got, want) {
		t.Fatalf("order after rejected move = %v, want %v", got, want)
	}
}

func TestSetupGuideRendersOrderedSteps(t *testing.T) {
	f := newSetupWebFixture(t)
	f.addImage(t, "img-1", "Alpha")
	f.addImage(t, "img-2", "Beta")

	rec := f.move(t, "move_up", "img-2", f.owner)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("move up status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	target := "/games/" + strconv.FormatInt(f.gameID, 10)
	req := f.newRequest(t, http.MethodGet, target, "", nil, f.owner)
	rec = httptest.NewRecorder()
	f.handler.gameDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("game detail status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	stepOne := strings.Index(body, "Step 1")
	beta := strings.Index(body, `setup-caption">Beta`)
	stepTwo := strings.Index(body, "Step 2")
	alpha := strings.Index(body, `setup-caption">Alpha`)
	if stepOne < 0 || stepTwo < 0 || beta < 0 || alpha < 0 {
		t.Fatalf("rendered guide is missing step labels or captions: %s", body)
	}
	if !(stepOne < beta && beta < stepTwo && stepTwo < alpha) {
		t.Fatalf("rendered order is not Step 1 Beta then Step 2 Alpha (positions %d %d %d %d)", stepOne, beta, stepTwo, alpha)
	}
	if strings.Contains(body, "Step 2 —") {
		t.Fatal("empty-caption separator leaked into the rendered guide")
	}
}

func assertPositions(t *testing.T, database *sql.DB, userID string, gameID int64) {
	t.Helper()
	rows, err := database.Query(`SELECT position FROM game_setup_images WHERE user_id=? AND game_id=? ORDER BY position`, userID, gameID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}
	for i, p := range got {
		if p != i {
			t.Fatalf("positions = %v, want contiguous 0..n-1", got)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
