package web

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"boardgames/internal/collection"
	"boardgames/internal/config"
	"boardgames/internal/plays"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

var (
	tmplMu    sync.RWMutex
	tmplCache = map[string]*template.Template{}
)

var tagRE = regexp.MustCompile(`<[^>]*>`)
var spaceRE = regexp.MustCompile(`\s+`)
var setupLinkRE = regexp.MustCompile(`https?://[^\s<>"']+`)

// funcMap builds the template helpers for one instance currency. Passing the
// currency keeps the helpers pure, so the template cache can be keyed by it.
func funcMap(currencies ...string) template.FuncMap {
	currency := "GBP"
	if len(currencies) > 0 && strings.TrimSpace(currencies[0]) != "" {
		currency = currencies[0]
	}
	symbol := currencySymbol(currency)
	return template.FuncMap{
		"asset":     Asset,
		"assetCSS":  AssetCSS,
		"plain":     PlainText,
		"setupText": SetupText,
		"truncate":  truncateText,
		"dateLong":  dateLong,
		"dateShort": dateShort,
		"monthKey":  monthKey,
		"yearOf":    yearOf,
		"money":     func(cents *int) string { return money(cents, symbol) },
		"moneyInt": func(cents int) string {
			return money(&cents, symbol)
		},
		"moneyRate": func(cents *float64) string { return moneyRate(cents, symbol) },
		"currencySymbol": func() string {
			return strings.TrimSpace(symbol)
		},
		"pounds": func(cents *int) string {
			if cents == nil {
				return ""
			}
			return fmt.Sprintf("%.2f", float64(*cents)/100)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i + 1
			}
			return out
		},
		"pct": func(n, total int) int {
			if total <= 0 {
				return 0
			}
			return n * 100 / total
		},
		"initials": initials,
		"join":     func(v []string, sep string) string { return strings.Join(v, sep) },
		"half":     func(i int) float64 { return float64(i) / 2 },
		"eqf":      func(a, b float64) bool { return math.Abs(a-b) < 0.001 },
		"ratingSelected": func(v *float64, want float64) bool {
			return v != nil && math.Abs(*v-want) < 0.001
		},
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict needs pairs")
			}
			out := map[string]any{}
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict key must be a string")
				}
				out[key] = values[i+1]
			}
			return out, nil
		},
	}
}

func parsePage(page string, currencies ...string) (*template.Template, error) {
	currency := "GBP"
	if len(currencies) > 0 && strings.TrimSpace(currencies[0]) != "" {
		currency = currencies[0]
	}
	key := page + "\x00" + strings.ToUpper(currency)
	tmplMu.RLock()
	t, ok := tmplCache[key]
	tmplMu.RUnlock()
	if ok {
		return t, nil
	}
	t, err := template.New("base").Funcs(funcMap(currency)).ParseFS(templateFS, "templates/base.html", "templates/partials/*.html", "templates/"+page)
	if err != nil {
		return nil, err
	}
	tmplMu.Lock()
	tmplCache[key] = t
	tmplMu.Unlock()
	return t, nil
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, page string, p Page) {
	t, err := parsePage(page, h.Config.ShopCurrency)
	if err != nil {
		slog.Error("parse template", "page", page, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", p); err != nil {
		slog.Error("execute template", "page", page, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) renderPartial(w http.ResponseWriter, name string, data any) {
	currency := h.Config.ShopCurrency
	if strings.TrimSpace(currency) == "" {
		currency = "GBP"
	}
	key := "\x00partials\x00" + strings.ToUpper(currency)
	tmplMu.RLock()
	t, ok := tmplCache[key]
	tmplMu.RUnlock()
	if !ok {
		var err error
		t, err = template.New("partials").Funcs(funcMap(currency)).ParseFS(templateFS, "templates/partials/*.html")
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		tmplMu.Lock()
		tmplCache[key] = t
		tmplMu.Unlock()
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("execute partial", "name", name, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// PlainText converts BGG's escaped HTML descriptions into readable prose.
func PlainText(s string) string {
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br />", " ")
	s = strings.ReplaceAll(s, "</p>", " ")
	s = tagRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

// SetupText renders a setup guide's plain text as safe HTML. All user text is
// escaped before links are added, so no raw markup can survive.
func SetupText(s string) template.HTML {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(html.EscapeString(s), "\n")
	isBullet := func(line string) bool {
		return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")
	}
	var out strings.Builder
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if isBullet(line) {
			out.WriteString("<ul>")
			for i < len(lines) {
				item := strings.TrimSpace(lines[i])
				if !isBullet(item) {
					break
				}
				out.WriteString("<li>")
				out.WriteString(setupLinkify(strings.TrimSpace(item[1:])))
				out.WriteString("</li>")
				i++
			}
			out.WriteString("</ul>")
			continue
		}
		var parts []string
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if next == "" || isBullet(next) {
				break
			}
			parts = append(parts, next)
			i++
		}
		out.WriteString("<p>")
		out.WriteString(setupLinkify(strings.Join(parts, " ")))
		out.WriteString("</p>")
	}
	return template.HTML(out.String())
}

func setupLinkify(s string) string {
	return setupLinkRE.ReplaceAllStringFunc(s, func(match string) string {
		url := strings.TrimRight(match, ".,;:!?)]")
		suffix := match[len(url):]
		return `<a href="` + url + `" target="_blank" rel="noopener noreferrer nofollow">` + url + `</a>` + suffix
	})
}

func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

func dateLong(s string) string {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2 January 2006")
		}
	}
	return s
}

func dateShort(s string) string {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2 Jan 2006")
		}
	}
	return s
}

func monthKey(s string) string {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("January 2006")
		}
	}
	return "Undated"
}

func yearOf(s string) string {
	if len(s) >= 4 {
		return s[:4]
	}
	return s
}

// currencySymbol maps the instance currency code to a display prefix. It is a
// thin alias over config.CurrencySymbol so templates and generated copy share
// one mapping.
func currencySymbol(code string) string {
	return config.CurrencySymbol(code)
}

func money(cents *int, symbol string) string {
	if cents == nil {
		return ""
	}
	return fmt.Sprintf("%s%d.%02d", symbol, *cents/100, *cents%100)
}

// moneyRate formats a fractional minor-unit value, such as a derived cost per
// play, with exactly two decimals.
func moneyRate(cents *float64, symbol string) string {
	if cents == nil {
		return ""
	}
	return fmt.Sprintf("%s%.2f", symbol, *cents/100)
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	out := strings.ToUpper(string([]rune(parts[0])[:1]))
	if len(parts) > 1 {
		out += strings.ToUpper(string([]rune(parts[len(parts)-1])[:1]))
	}
	return out
}

// PlayGroup is a month of plays for the timeline.
type PlayGroup struct {
	Month string
	Plays []plays.Play
}

func groupPlays(list []plays.Play) []PlayGroup {
	var groups []PlayGroup
	for _, p := range list {
		key := monthKey(p.PlayedAt)
		if len(groups) == 0 || groups[len(groups)-1].Month != key {
			groups = append(groups, PlayGroup{Month: key})
		}
		groups[len(groups)-1].Plays = append(groups[len(groups)-1].Plays, p)
	}
	return groups
}

// QueryURL lets templates build shareable filter URLs.
func (d *OverviewData) QueryURL(page int) string { return d.queryURL(page) }

func (d *OverviewData) HasPrev() bool { return d.Filters.Page > 1 }
func (d *OverviewData) HasNext() bool { return d.Filters.Page*pageSize(d.Filters) < d.Total }

func pageSize(f collection.Filters) int {
	if f.PerPage <= 0 {
		return 48
	}
	return f.PerPage
}

// WithParam returns the current shelf URL with one filter changed.
func (d *OverviewData) WithParam(key, value string) string {
	u, _ := url.Parse(d.queryURL(d.Filters.Page))
	q := u.Query()
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	q.Del("page")
	u.RawQuery = q.Encode()
	return u.String()
}

// PageURL returns a pagination URL preserving filters.
func (d *OverviewData) PageURL(page int) string { return d.queryURL(page) }
