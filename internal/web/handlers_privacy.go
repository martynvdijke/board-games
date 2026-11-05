package web

import (
	"net/http"
	"time"

	"boardgames/internal/auth"
)

// savePrivacy records the analytics opt-out preference from the settings page.
func (h *Handler) savePrivacy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 10*time.Second)
	defer cancel()
	user := auth.UserFromContext(r.Context())
	_ = r.ParseForm()
	optOut := r.PostForm.Get("analytics_opt_out") == "1"
	if err := h.Auth.SetAnalyticsOptOut(ctx, user.ID, optOut); err != nil {
		h.flashRedirect(w, r, "/settings#privacy-controls", "Privacy settings couldn't be saved.", "error")
		return
	}
	message := "Analytics enabled."
	if optOut {
		message = "Analytics disabled."
	}
	h.flashRedirect(w, r, "/settings#privacy-controls", message, "success")
}
