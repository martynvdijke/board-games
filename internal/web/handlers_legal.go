package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"boardgames/internal/account"
	"boardgames/internal/auth"
)

// LegalData is the operator identity and the deployment facts the published
// policy is allowed to assert. Each flag follows configuration, so the text
// cannot claim a processor that is not running on this instance.
type LegalData struct {
	Controller     string
	ContactEmail   string
	ContactAddress string
	HostingRegion  string
	BaseURL        string
	SessionCookie  string
	OIDC           bool
	Analytics      bool
	AnalyticsURL   string
	Traces         bool
	TRMNL          bool
}

func (h *Handler) legalData() LegalData {
	cfg := h.Config
	d := LegalData{
		Controller:    "The operator of this " + cfg.AppName + " instance",
		HostingRegion: "the European Union",
		BaseURL:       cfg.BaseURL,
		SessionCookie: auth.SessionCookie,
		OIDC:          cfg.OIDCIssuer != "",
		Analytics:     cfg.UmamiURL != "",
		AnalyticsURL:  cfg.UmamiURL,
		Traces:        cfg.OTLPEndpoint != "",
	}
	if v := strings.TrimSpace(cfg.LegalControllerName); v != "" {
		d.Controller = v
	}
	d.ContactEmail = strings.TrimSpace(cfg.LegalContactEmail)
	d.ContactAddress = strings.TrimSpace(cfg.LegalContactAddress)
	if v := strings.TrimSpace(cfg.LegalHostingRegion); v != "" {
		d.HostingRegion = v
	}
	return d
}

func (h *Handler) privacyPage(w http.ResponseWriter, r *http.Request) {
	p := h.page(r, "Privacy", "", nil)
	h.render(w, r, "privacy.html", p)
}

func (h *Handler) termsPage(w http.ResponseWriter, r *http.Request) {
	p := h.page(r, "Terms", "", nil)
	h.render(w, r, "terms.html", p)
}

func (h *Handler) accountDeletePage(w http.ResponseWriter, r *http.Request) {
	p := h.page(r, "Delete your account", "", nil)
	h.render(w, r, "account_delete.html", p)
}

// deleteAccount closes the signed-in user's account. The CSRF middleware has
// already validated the form token; the typed confirmation makes an accidental
// click impossible.
func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 20*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	if !strings.EqualFold(strings.TrimSpace(r.PostForm.Get("confirm")), "DELETE") {
		h.flashRedirect(w, r, "/settings", `Type DELETE to confirm account deletion.`, "error")
		return
	}
	if h.Account == nil {
		h.flashRedirect(w, r, "/settings", "Account deletion is not configured.", "error")
		return
	}
	err := h.Account.Delete(ctx, user.ID)
	switch {
	case errors.Is(err, account.ErrLastAdmin):
		h.flashRedirect(w, r, "/settings", "This is the last admin account. Promote another admin before deleting it.", "error")
		return
	case err != nil:
		h.flashRedirect(w, r, "/settings", "The account could not be deleted. Try again or contact the operator.", "error")
		return
	}
	h.Auth.ClearSessionCookie(w)
	h.flashRedirect(w, r, "/account/delete", "Your account has been deleted.", "success")
}

// removePlayerName lets a user remove or anonymise a third-party name from the
// plays they own. An admin may apply it across the instance; nobody can target
// another user's plays.
func (h *Handler) removePlayerName(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	name := strings.TrimSpace(r.PostForm.Get("name"))
	mode := strings.TrimSpace(r.PostForm.Get("mode"))
	anonymise := mode == "anonymise"
	if name == "" {
		h.flashRedirect(w, r, "/settings#privacy-controls", "Enter the player name to remove.", "error")
		return
	}
	if h.Account == nil {
		h.flashRedirect(w, r, "/settings#privacy-controls", "Player-name removal is not configured.", "error")
		return
	}
	n, err := h.Account.RemovePlayerName(ctx, user.ID, user.Role == "admin", name, anonymise)
	if err != nil {
		h.flashRedirect(w, r, "/settings#privacy-controls", "The player name could not be removed.", "error")
		return
	}
	if n == 0 {
		h.flashRedirect(w, r, "/settings#privacy-controls", "No matching player name was found in your plays.", "error")
		return
	}
	action := "removed"
	if anonymise {
		action = "anonymised"
	}
	h.flashRedirect(w, r, "/settings#privacy-controls", "Player name "+action+" on "+strconv.FormatInt(n, 10)+" row(s).", "success")
}
