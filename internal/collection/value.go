package collection

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"boardgames/internal/config"
)

// TradeCandidate is one ranked trade suggestion: the game, its score, and a
// short human reason for the rank.
type TradeCandidate struct {
	Game   GameCard
	Score  float64
	Reason string
}

// RankTradeCandidates ranks games as trade candidates using the current time.
// The optional currency code (ISO 4217) drives the symbol in the reason text
// and defaults to GBP. It delegates to RankTradeCandidatesAt with time.Now() so
// the ranking itself stays pure and testable.
func RankTradeCandidates(games []GameCard, currencies ...string) []TradeCandidate {
	return RankTradeCandidatesAt(games, time.Now(), currencies...)
}

// RankTradeCandidatesAt is the pure, clock-injectable form of
// RankTradeCandidates. It performs no I/O and is deterministic for a given
// input, now and currency. Results are sorted by descending score, then by
// name, then by id ascending so equal scores are stable.
func RankTradeCandidatesAt(games []GameCard, now time.Time, currencies ...string) []TradeCandidate {
	currency := "GBP"
	if len(currencies) > 0 && strings.TrimSpace(currencies[0]) != "" {
		currency = currencies[0]
	}
	symbol := config.CurrencySymbol(currency)
	out := make([]TradeCandidate, 0, len(games))
	for _, g := range games {
		out = append(out, rankTradeCandidate(g, now, symbol))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Game.Name != out[j].Game.Name {
			return out[i].Game.Name < out[j].Game.Name
		}
		return out[i].Game.ID < out[j].Game.ID
	})
	return out
}

// rankTradeCandidate scores one game. A game is a better trade candidate when
// it is worth more money, has been played fewer times relative to how long it
// has been owned, and is not a favourite.
//
//	value   = min(priceGBP, 100) / 100 * 40     (one expensive game cannot dominate)
//	neglect = 40 * (1 / (1 + plays)) * min(yearsOwned, 5) / 5
//	score   = (value + neglect), rounded to 2 decimals
//
// A favourite scores zero: it is kept regardless of the other signals.
func rankTradeCandidate(g GameCard, now time.Time, symbol string) TradeCandidate {
	if g.Favorite {
		return TradeCandidate{Game: g, Score: 0, Reason: "Favourite — keep it"}
	}

	pricePence := 0
	if g.PricePaid != nil && *g.PricePaid > 0 {
		pricePence = *g.PricePaid
	}
	priceGBP := float64(pricePence) / 100

	plays := g.TotalPlays

	since := g.AcquisitionDate
	if since == "" {
		since = g.AddedAt
	}
	var yearsOwned float64
	if t, ok := parseDate(since); ok {
		if days := now.Sub(t).Hours() / 24; days > 0 {
			yearsOwned = days / 365.25
		}
	}
	wholeYears := int(yearsOwned)

	// Value signal, capped so one expensive game cannot dominate.
	score := math.Min(priceGBP, 100) / 100 * 40

	// Neglect signal: fewer plays and longer ownership score higher. When the
	// date is missing or unparseable this contributes 0 rather than guessing.
	if yearsOwned > 0 {
		ownership := math.Min(yearsOwned, 5) / 5
		playFactor := 1 / (1 + float64(plays))
		score += 40 * playFactor * ownership
	}

	score = math.Round(score*100) / 100

	return TradeCandidate{
		Game:   g,
		Score:  score,
		Reason: tradeReason(priceGBP, plays, wholeYears, symbol),
	}
}

// tradeReason builds the short human sentence shown beside a candidate. It is
// never empty: when no signal fires it falls back to a plain statement.
func tradeReason(priceGBP float64, plays, years int, symbol string) string {
	var neglect string
	switch {
	case years >= 1 && plays == 0:
		neglect = "owned " + yearWord(years) + ", never played"
	case years >= 1:
		neglect = "owned " + yearWord(years) + ", " + playPhrase(plays)
	case plays == 0:
		neglect = "never played since you added it"
	default:
		neglect = playPhrase(plays)
	}

	reason := neglect
	if priceGBP > 0 {
		prefix := symbol + formatAmount(priceGBP)
		if plays == 0 {
			prefix += " shelf-warming"
		} else {
			prefix += " spent"
		}
		reason = prefix + ", " + neglect
	}
	return capitalize(reason)
}

func yearWord(n int) string {
	if n == 1 {
		return "1 year"
	}
	return itoa(n) + " years"
}

func playPhrase(plays int) string {
	switch plays {
	case 0:
		return "never played"
	case 1:
		return "played once"
	case 2:
		return "played twice"
	default:
		return "played " + itoa(plays) + " times"
	}
}

func formatAmount(v float64) string {
	if v == math.Trunc(v) {
		return itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}
