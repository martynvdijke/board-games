package web

import (
	"testing"
)

func TestPagesParse(t *testing.T) {
	pages := []string{"login.html", "overview.html", "game.html", "plays.html", "play_form.html", "stats.html", "settings.html", "trmnl_preview.html", "error.html"}
	for _, page := range pages {
		if _, err := parsePage(page); err != nil {
			t.Errorf("%s: %v", page, err)
		}
	}
}

func TestPartialsParse(t *testing.T) {
	if _, err := parsePage("overview.html"); err != nil {
		t.Fatalf("parse overview: %v", err)
	}
}

func TestAssetFallbacks(t *testing.T) {
	if got := Asset("src/main.ts"); got == "" {
		t.Fatal("asset path is empty")
	}
	if got := AssetCSS("src/main.ts"); len(got) == 0 {
		t.Fatal("asset css is empty")
	}
}
