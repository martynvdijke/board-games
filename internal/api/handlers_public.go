package api

import (
	"net/http"
)

// authConfig exposes the optional demo account to the companion app. It is
// public by design but never cacheable, because the credential can rotate.
func (h *Handler) authConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	demo := map[string]any{"enabled": h.Config.DemoUserEnabled}
	if h.Config.DemoUserEnabled {
		demo["email"] = h.Config.DemoUserEmail
		demo["name"] = h.Config.DemoUserName
		demo["token"] = h.Config.DemoUserToken
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"demo": demo}})
}
