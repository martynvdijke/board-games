package api

import (
	"net/http"
	"strings"
)

// otelSettings exposes the server's OpenTelemetry collector configuration to
// the companion app. It is public by design: a fresh install needs the OTLP
// endpoint before it has a token. The response is never cacheable because the
// operator can rotate the collector without an app release.
func (h *Handler) otelSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	endpoint := strings.TrimSpace(h.Config.OTLPEndpoint)
	enabled := endpoint != ""
	writeJSON(w, http.StatusOK, map[string]any{
		"endpoint":        endpoint,
		"enabled":         enabled,
		"traces_enabled":  enabled,
		"metrics_enabled": false,
		"logs_enabled":    false,
		"service_name":    h.Config.OTELServiceName,
	})
}
