package bgg

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientPolls202AndRetries429(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusAccepted)
		case 2:
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			if r.Header.Get("User-Agent") == "" {
				t.Error("missing user agent")
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><items total="0"></items>`))
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "TheShelfTest/1.0 (test@example.com)", 100)
	body, err := client.Get(context.Background(), "collection", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" || calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", calls.Load())
	}
}

func TestNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := NewClient(server.URL, "TheShelfTest/1.0 (test@example.com)", 100)
	if _, err := client.Get(context.Background(), "thing", nil); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCollectionParsing(t *testing.T) {
	xmlData := `<items total="1"><item objecttype="thing" objectid="13" subtype="boardgame"><name>Catan</name><yearpublished>1995</yearpublished><stats minplayers="3" maxplayers="4" minplaytime="60" maxplaytime="120" playingtime="120" minage="10"><rating value="8"/></stats><status own="1" wishlist="0" wanttoplay="0"/><numplays>2</numplays></item></items>`
	var c Collection
	if err := xml.Unmarshal([]byte(xmlData), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 1 || !c.Items[0].Status.IsOwned() || c.Items[0].Stats.Rating.Personal() == nil || *c.Items[0].Stats.Rating.Personal() != 8 {
		t.Fatalf("unexpected collection: %+v", c)
	}
}

func TestRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<items/>`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "TheShelfTest/1.0 (test@example.com)", 20)
	start := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := client.Get(context.Background(), "thing", nil); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("rate limiter did not delay: %s", elapsed)
	}
}
