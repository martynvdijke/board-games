package api

import (
	"net/http"
	"testing"
)

func TestOtelSettingsIsPublicAndReportsConfiguredEndpoint(t *testing.T) {
	f := setupAPI(t)
	f.handler.Config.OTLPEndpoint = "http://collector.internal:4318"
	f.handler.Config.OTELServiceName = "the-shelf-api"

	rr := f.request(t, http.MethodGet, "/v1/otel-settings", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("otel settings: %d %s", rr.Code, rr.Body)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	body := decode(t, rr)
	if body["endpoint"] != f.handler.Config.OTLPEndpoint {
		t.Errorf("endpoint = %v, want %q", body["endpoint"], f.handler.Config.OTLPEndpoint)
	}
	if body["enabled"] != true {
		t.Errorf("enabled = %v, want true", body["enabled"])
	}
	if body["traces_enabled"] != true {
		t.Errorf("traces_enabled = %v, want true", body["traces_enabled"])
	}
	if body["metrics_enabled"] != false {
		t.Errorf("metrics_enabled = %v, want false", body["metrics_enabled"])
	}
	if body["logs_enabled"] != false {
		t.Errorf("logs_enabled = %v, want false", body["logs_enabled"])
	}
	if body["service_name"] != f.handler.Config.OTELServiceName {
		t.Errorf("service_name = %v, want %q", body["service_name"], f.handler.Config.OTELServiceName)
	}
}

func TestOtelSettingsDisabledWhenEndpointEmpty(t *testing.T) {
	f := setupAPI(t)
	f.handler.Config.OTLPEndpoint = ""
	f.handler.Config.OTELServiceName = "the-shelf-api"

	rr := f.request(t, http.MethodGet, "/v1/otel-settings", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("otel settings: %d %s", rr.Code, rr.Body)
	}
	body := decode(t, rr)
	if body["endpoint"] != "" {
		t.Errorf("endpoint = %v, want empty", body["endpoint"])
	}
	if body["enabled"] != false {
		t.Errorf("enabled = %v, want false", body["enabled"])
	}
	if body["traces_enabled"] != false {
		t.Errorf("traces_enabled = %v, want false", body["traces_enabled"])
	}
	if body["service_name"] != f.handler.Config.OTELServiceName {
		t.Errorf("service_name = %v, want %q", body["service_name"], f.handler.Config.OTELServiceName)
	}
}
