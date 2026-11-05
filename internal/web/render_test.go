package web

import (
	"testing"
)

func TestPagesParse(t *testing.T) {
	pages := []string{"login.html", "overview.html", "game.html", "plays.html", "play_form.html", "stats.html", "settings.html", "trmnl_preview.html", "error.html", "privacy.html", "terms.html", "account_delete.html"}
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

func TestSetupTextEscapesHTML(t *testing.T) {
	got := string(SetupText(`<script>alert("x")</script>`))
	want := `<p>&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;</p>`
	if got != want {
		t.Fatalf("SetupText = %q, want %q", got, want)
	}
}

func TestSetupTextLinkifies(t *testing.T) {
	got := string(SetupText("See https://example.com/path."))
	want := `<p>See <a href="https://example.com/path" target="_blank" rel="noopener noreferrer nofollow">https://example.com/path</a>.</p>`
	if got != want {
		t.Fatalf("SetupText = %q, want %q", got, want)
	}
}

func TestSetupTextBullets(t *testing.T) {
	got := string(SetupText("- One\n* Two"))
	want := `<ul><li>One</li><li>Two</li></ul>`
	if got != want {
		t.Fatalf("SetupText = %q, want %q", got, want)
	}
}

func TestSetupTextParagraphs(t *testing.T) {
	got := string(SetupText("First line\r\nsecond line\r\n\r\nThird"))
	want := `<p>First line second line</p><p>Third</p>`
	if got != want {
		t.Fatalf("SetupText = %q, want %q", got, want)
	}
}
