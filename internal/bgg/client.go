package bgg

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"boardgames/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const defaultBaseURL = "https://boardgamegeek.com/xmlapi2"

type Client struct {
	BaseURL     string
	UserAgent   string
	HTTP        *http.Client
	MaxAttempts int
	// MinInterval controls the process-wide request spacing.
	MinInterval time.Duration

	mu   sync.Mutex
	next time.Time

	tokenMu sync.RWMutex
	token   string
}

func NewClient(baseURL, userAgent string, requestsPerSecond float64) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	// Go drops Authorization on cross-domain redirects, and BGG redirects www
	// to the apex host, so normalise defensively here too.
	baseURL = strings.Replace(baseURL, "://www.boardgamegeek.com", "://boardgamegeek.com", 1)
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	return &Client{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		UserAgent:   userAgent,
		HTTP:        &http.Client{Timeout: 45 * time.Second},
		MaxAttempts: 6,
		MinInterval: time.Duration(float64(time.Second) / requestsPerSecond),
	}
}

// SetToken replaces the application API token used for subsequent requests.
func (c *Client) SetToken(token string) {
	c.tokenMu.Lock()
	c.token = strings.TrimSpace(token)
	c.tokenMu.Unlock()
}

func (c *Client) currentToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.token
}

// Get fetches a path with retries for BGG's queue, throttle, and outage signals.
func (c *Client) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.get(ctx, path, query, c.currentToken())
}

func (c *Client) get(ctx context.Context, path string, query url.Values, token string) ([]byte, error) {
	endpoint := strings.TrimLeft(path, "/")
	ctx, span := otel.Tracer("bgg").Start(ctx, "bgg "+endpoint,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("bgg.endpoint", endpoint)))
	var attempts int
	var succeeded bool
	defer func() {
		span.SetAttributes(attribute.Int("bgg.attempts", attempts))
		if !succeeded {
			span.SetStatus(codes.Error, "BoardGameGeek request failed")
		}
		span.End()
	}()
	u := c.BaseURL + "/" + endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var lastErr error
	for attempt := 1; attempt <= c.MaxAttempts; attempt++ {
		attempts = attempt
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/xml, text/xml;q=0.9, */*;q=0.1")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			observability.RecordBGGRequest(ctx, endpoint, 0)
			lastErr = err
			if err := sleepJitter(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		observability.RecordBGGRequest(ctx, endpoint, resp.StatusCode)
		span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if err := sleepJitter(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			if msg, ok := errorDocument(body); ok {
				return nil, fmt.Errorf("%w: %s", ErrNotFound, msg)
			}
			succeeded = true
			return body, nil
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			return nil, ErrUnauthorized
		case resp.StatusCode == http.StatusAccepted, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusServiceUnavailable:
			lastErr = fmt.Errorf("BGG is busy (HTTP %d)", resp.StatusCode)
			delay := retryDelay(resp, attempt)
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
		case resp.StatusCode == http.StatusNotFound:
			return nil, ErrNotFound
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			return nil, fmt.Errorf("BGG rejected the request (HTTP %d): %s", resp.StatusCode, snippet(body))
		default:
			lastErr = fmt.Errorf("BGG returned HTTP %d: %s", resp.StatusCode, snippet(body))
			if err := sleepJitter(ctx, attempt); err != nil {
				return nil, err
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("BGG request failed")
	}
	return nil, fmt.Errorf("BGG request did not succeed after %d attempts: %w", c.MaxAttempts, lastErr)
}

var ErrNotFound = errors.New("BoardGameGeek resource not found")

// ErrUnauthorized means BGG rejected the request's application API token.
var ErrUnauthorized = errors.New("BoardGameGeek requires a valid API token; add one in Settings")

// errorDocument detects BGG's XML error envelope, which is served with HTTP 200
// for some failures such as unknown or private usernames.
func errorDocument(body []byte) (string, bool) {
	var doc struct {
		Errors []struct {
			Message string `xml:"message"`
		} `xml:"error"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil || len(doc.Errors) == 0 {
		return "", false
	}
	msg := strings.TrimSpace(doc.Errors[0].Message)
	if msg == "" {
		msg = "BoardGameGeek returned an error"
	}
	return msg, true
}

// ValidateToken checks the supplied application token without changing the
// client's configured token.
func (c *Client) ValidateToken(ctx context.Context, token string) error {
	_, err := c.get(ctx, "thing", url.Values{"id": {"13"}}, strings.TrimSpace(token))
	return err
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	wait := c.next.Sub(now)
	if wait < 0 {
		wait = 0
	}
	c.next = now.Add(wait).Add(c.MinInterval)
	c.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	return sleepContext(ctx, wait)
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 && secs <= 120 {
			return time.Duration(secs) * time.Second
		}
	}
	base := time.Duration(1<<uint(min(attempt, 5))) * time.Second
	return base + time.Duration(rand.IntN(700))*time.Millisecond
}

func sleepJitter(ctx context.Context, attempt int) error {
	base := time.Duration(1<<uint(min(attempt, 4))) * 250 * time.Millisecond
	return sleepContext(ctx, base+time.Duration(rand.IntN(250))*time.Millisecond)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func (c *Client) Collection(ctx context.Context, username string) (*Collection, error) {
	body, err := c.Get(ctx, "collection", url.Values{"username": {username}, "stats": {"1"}})
	if err != nil {
		return nil, err
	}
	var out Collection
	if err := xml.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse BGG collection: %w", err)
	}
	return &out, nil
}

func (c *Client) Things(ctx context.Context, ids []int) (*Things, error) {
	if len(ids) == 0 {
		return &Things{}, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	body, err := c.Get(ctx, "thing", url.Values{"id": {strings.Join(parts, ",")}, "stats": {"1"}})
	if err != nil {
		return nil, err
	}
	var out Things
	if err := xml.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse BGG thing: %w", err)
	}
	return &out, nil
}

func (c *Client) Plays(ctx context.Context, username string, page int) (*Plays, error) {
	if page < 1 {
		page = 1
	}
	body, err := c.Get(ctx, "plays", url.Values{"username": {username}, "page": {strconv.Itoa(page)}})
	if err != nil {
		return nil, err
	}
	var out Plays
	if err := xml.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse BGG plays: %w", err)
	}
	return &out, nil
}

// ValidateUsername checks that a collection is public and non-empty.
func (c *Client) ValidateUsername(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	_, err := c.Collection(ctx, username)
	if errors.Is(err, ErrNotFound) {
		return errors.New("that collection is private or does not exist; only public BoardGameGeek collections can be synced")
	}
	if err != nil {
		return err
	}
	return nil
}
