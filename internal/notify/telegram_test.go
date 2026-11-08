package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTelegramDisabledWhenUnconfigured(t *testing.T) {
	cases := []struct{ token, chat string }{
		{"", "123"},
		{"token", ""},
		{"", ""},
		{"   ", "  "},
	}
	for _, c := range cases {
		if got := NewTelegram(c.token, c.chat); got != nil {
			t.Errorf("NewTelegram(%q, %q) = %#v, want nil", c.token, c.chat, got)
		}
	}
}

func TestTelegramNotifySuccess(t *testing.T) {
	var gotPath, gotMethod, gotContentType string
	var gotBody sendMessageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tg := NewTelegram("secret-token", "chat-42")
	tg.BaseURL = srv.URL
	tg.Client = srv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tg.Notify(ctx, "New play logged", "Catan — 2026-01-02 · 2 players"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/botsecret-token/sendMessage" {
		t.Errorf("path = %q, want /botsecret-token/sendMessage", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if gotBody.ChatID != "chat-42" {
		t.Errorf("chat_id = %q, want chat-42", gotBody.ChatID)
	}
	if !strings.Contains(gotBody.Text, "New play logged") || !strings.Contains(gotBody.Text, "Catan") {
		t.Errorf("text = %q, want title and body", gotBody.Text)
	}
	if !gotBody.DisableWebPagePreview {
		t.Error("disable_web_page_preview should be true")
	}
}

func TestTelegramNotifyHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false,"description":"boom"}`))
	}))
	defer srv.Close()

	tg := NewTelegram("token", "chat")
	tg.BaseURL = srv.URL
	tg.Client = srv.Client()

	if err := tg.Notify(context.Background(), "title", "body"); err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
}

func TestTelegramNotifyNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer srv.Close()

	tg := NewTelegram("token", "chat")
	tg.BaseURL = srv.URL
	tg.Client = srv.Client()

	err := tg.Notify(context.Background(), "title", "body")
	if err == nil {
		t.Fatal("expected an error when ok is false")
	}
	if strings.Contains(err.Error(), "token") {
		t.Errorf("error leaked the token: %v", err)
	}
}

func TestTelegramNotifyEmptyFields(t *testing.T) {
	tg := &Telegram{BaseURL: DefaultBaseURL}
	if err := tg.Notify(context.Background(), "title", "body"); err == nil {
		t.Fatal("expected an error with an empty token/chat id")
	}
	tg = &Telegram{BotToken: "token", ChatID: "  "}
	if err := tg.Notify(context.Background(), "title", "body"); err == nil {
		t.Fatal("expected an error with an empty chat id")
	}
}

func TestNilTelegramNotifyIsNoop(t *testing.T) {
	var tg *Telegram
	if err := tg.Notify(context.Background(), "title", "body"); err != nil {
		t.Fatalf("nil notifier should be a no-op, got %v", err)
	}
}
