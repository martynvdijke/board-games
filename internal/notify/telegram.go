package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is Telegram's Bot API root.
const DefaultBaseURL = "https://api.telegram.org"

// requestTimeout bounds a single sendMessage call.
const requestTimeout = 10 * time.Second

// Telegram delivers notifications through a Telegram bot's sendMessage method.
// Plain text is used, so message content is never interpreted as Markdown.
type Telegram struct {
	BotToken string
	ChatID   string
	// BaseURL is overridable in tests; it defaults to DefaultBaseURL.
	BaseURL string
	// Client is overridable in tests; a 10s-timeout client is the default.
	Client *http.Client
}

// NewTelegram returns a notifier, or nil when either credential is blank so
// callers can treat nil as "notifications disabled".
func NewTelegram(botToken, chatID string) *Telegram {
	token := strings.TrimSpace(botToken)
	chat := strings.TrimSpace(chatID)
	if token == "" || chat == "" {
		return nil
	}
	return &Telegram{
		BotToken: token,
		ChatID:   chat,
		BaseURL:  DefaultBaseURL,
		Client:   &http.Client{Timeout: requestTimeout},
	}
}

type sendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (t *Telegram) Notify(ctx context.Context, title, body string) error {
	if t == nil {
		return nil
	}
	token := strings.TrimSpace(t.BotToken)
	chat := strings.TrimSpace(t.ChatID)
	if token == "" || chat == "" {
		return errors.New("telegram: bot token and chat id are required")
	}
	text := strings.TrimSpace(title)
	if b := strings.TrimSpace(body); b != "" {
		if text != "" {
			text += "\n" + b
		} else {
			text = b
		}
	}
	payload, err := json.Marshal(sendMessageRequest{
		ChatID:                chat,
		Text:                  text,
		DisableWebPagePreview: true,
	})
	if err != nil {
		return fmt.Errorf("telegram: encode message: %w", err)
	}

	base := strings.TrimRight(t.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	endpoint := base + "/bot" + token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		// The token is part of the URL, so never wrap the URL in the error.
		return errors.New("telegram: build request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Client errors can embed the request URL (and thus the token); strip it.
		return errors.New("telegram: request failed")
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var decoded sendMessageResponse
	_ = json.Unmarshal(raw, &decoded)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Description != "" {
			return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, decoded.Description)
		}
		return fmt.Errorf("telegram: status %d", resp.StatusCode)
	}
	if !decoded.OK {
		if decoded.Description != "" {
			return fmt.Errorf("telegram: not ok: %s", decoded.Description)
		}
		return errors.New("telegram: response was not ok")
	}
	return nil
}
