package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"boardgames/internal/auth"
	"boardgames/internal/collection"
	"boardgames/internal/loans"
)

const dateOnly = "2006-01-02"

// loansPage lists the games currently out, oldest lending date first, with the
// returned history underneath.
func (h *Handler) loansPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	open, err := h.Loans.Open(ctx, user.ID)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your loans", err)
		return
	}
	closed, err := h.Loans.Closed(ctx, user.ID, 50)
	if err != nil {
		h.errorPage(w, r, http.StatusInternalServerError, "We couldn't load your returned loans", err)
		return
	}
	p := h.page(r, "Loans", "loans", "")
	p.Data = &LoansData{Open: open, Closed: closed}
	h.render(w, r, "loans.html", p)
}

// createLoan lends one game. Validation failures re-render the game page with
// the form filled in; a success returns to the game the action started from.
func (h *Handler) createLoan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	gameID, err := strconv.ParseInt(r.PostForm.Get("game_id"), 10, 64)
	if err != nil || gameID <= 0 {
		h.errorPage(w, r, http.StatusBadRequest, "That lend form was incomplete", err)
		return
	}
	attempt := lendAttempt{
		Borrower: strings.TrimSpace(r.PostForm.Get("borrower")),
		Contact:  strings.TrimSpace(r.PostForm.Get("contact")),
		LentAt:   strings.TrimSpace(r.PostForm.Get("lent_at")),
		DueAt:    strings.TrimSpace(r.PostForm.Get("due_at")),
		Notes:    strings.TrimSpace(r.PostForm.Get("notes")),
	}
	if attempt.LentAt == "" {
		attempt.LentAt = time.Now().Format(dateOnly)
	}
	loan, err := h.Loans.Lend(ctx, user.ID, loans.NewLoan{
		GameID:   gameID,
		Borrower: attempt.Borrower,
		Contact:  attempt.Contact,
		LentAt:   attempt.LentAt,
		DueAt:    attempt.DueAt,
		Notes:    attempt.Notes,
	})
	if err != nil {
		h.renderLendForm(w, r, gameID, attempt, loanMessage(err))
		return
	}
	h.flashRedirect(w, r, "/games/"+strconv.FormatInt(gameID, 10), "Lent to "+loan.Borrower, "success")
}

// returnLoan closes one open loan and returns to whichever page asked for it.
func (h *Handler) returnLoan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	next := safeNext(r.PostForm.Get("next"))
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.flashRedirect(w, r, next, "That loan isn't here", "error")
		return
	}
	loan, err := h.Loans.Return(ctx, user.ID, id, "")
	if err != nil {
		if errors.Is(err, loans.ErrNotFound) {
			h.flashRedirect(w, r, next, "That loan isn't here", "error")
			return
		}
		h.flashRedirect(w, r, next, "We couldn't mark that loan returned", "error")
		return
	}
	h.flashRedirect(w, r, next, loan.GameName+" is back on the shelf", "success")
}

// renderLendForm re-renders the game page with a lending error and the values
// the user already typed.
func (h *Handler) renderLendForm(w http.ResponseWriter, r *http.Request, gameID int64, attempt lendAttempt, message string) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	game, err := h.Games.Get(ctx, user.ID, gameID)
	if err != nil {
		h.errorPage(w, r, http.StatusNotFound, "That game isn't on this shelf", err)
		return
	}
	data := h.gameData(ctx, user.ID, game)
	data.Lend = attempt
	w.WriteHeader(http.StatusUnprocessableEntity)
	p := h.page(r, game.Name, "shelf", "")
	p.Error = message
	p.Data = data
	h.render(w, r, "game.html", p)
}

// gameData loads a game's loan state and recent borrowers for the detail page.
func (h *Handler) gameData(ctx context.Context, userID string, game *collection.GameDetail) *GameData {
	data := &GameData{Game: game, PlayToday: time.Now().Format(dateOnly)}
	if h.Loans == nil {
		return data
	}
	if loan, err := h.Loans.OpenByGame(ctx, userID, game.ID); err == nil {
		data.Loan = loan
	} else {
		slog.Warn("load game loan", "game", game.ID, "error", err)
	}
	if game.Owned && data.Loan == nil {
		if borrowers, err := h.Loans.RecentBorrowers(ctx, userID, 12); err == nil {
			data.Borrowers = borrowers
		} else {
			slog.Warn("load recent borrowers", "error", err)
		}
	}
	return data
}

// loansByGame fetches the open loans for one page of shelf cards in a single
// query, keyed by game id.
func (h *Handler) loansByGame(ctx context.Context, userID string, games []collection.GameCard) map[int64]*loans.Loan {
	if h.Loans == nil || len(games) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(games))
	for _, g := range games {
		ids = append(ids, g.ID)
	}
	out, err := h.Loans.OpenByGameIDs(ctx, userID, ids)
	if err != nil {
		slog.Warn("load shelf loans", "error", err)
		return nil
	}
	return out
}

// lendAttempt keeps what the user typed when a lend is refused.
type lendAttempt struct {
	Borrower string
	Contact  string
	LentAt   string
	DueAt    string
	Notes    string
}

// safeNext accepts only a local path so a return can go back where it started.
func safeNext(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.Contains(v, "\\") {
		return "/loans"
	}
	return v
}

// loanMessage turns a lifecycle error into a plain sentence. The repository's
// sentinel text already names the specific field that needs attention.
func loanMessage(err error) string {
	switch {
	case errors.Is(err, loans.ErrOpenLoanExists):
		return "That game is already out on loan. Mark the first loan returned before lending it again."
	case errors.Is(err, loans.ErrNotOwned):
		return "Only games you own can be lent."
	case errors.Is(err, loans.ErrInvalid):
		if detail := detailOf(err); detail != "" {
			return detail
		}
		return "Check the borrower's name and the dates, then try again."
	default:
		return "We couldn't record that loan. Nothing was changed."
	}
}

// detailOf keeps the sentence after the final colon, so "loans: invalid loan: a
// borrower name is required" reads as "A borrower name is required".
func detailOf(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(msg)
	return string(unicode.ToUpper(r)) + msg[size:] + "."
}
