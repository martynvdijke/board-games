package api

import (
	"net/http"
	"testing"
)

func TestAuthConfigReturnsDemoCredentialsWhenEnabled(t *testing.T) {
	f := setupAPI(t)
	f.handler.Config.DemoUserEnabled = true
	f.handler.Config.DemoUserEmail = "demo@theshelf.local"
	f.handler.Config.DemoUserName = "Shelf Demo"
	f.handler.Config.DemoUserToken = "bg_demo_test_token_1234567890"

	rr := f.request(t, http.MethodGet, "/v1/auth/config", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("auth config: %d %s", rr.Code, rr.Body)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	demo := decode(t, rr)["data"].(map[string]any)["demo"].(map[string]any)
	if demo["enabled"] != true {
		t.Errorf("enabled = %v, want true", demo["enabled"])
	}
	if demo["email"] != f.handler.Config.DemoUserEmail {
		t.Errorf("email = %v", demo["email"])
	}
	if demo["name"] != f.handler.Config.DemoUserName {
		t.Errorf("name = %v", demo["name"])
	}
	if demo["token"] != f.handler.Config.DemoUserToken {
		t.Errorf("token = %v", demo["token"])
	}
}

func TestAuthConfigOmitsTokenWhenDisabled(t *testing.T) {
	f := setupAPI(t)
	f.handler.Config.DemoUserEnabled = false
	f.handler.Config.DemoUserToken = "bg_demo_test_token_1234567890"

	rr := f.request(t, http.MethodGet, "/v1/auth/config", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("auth config: %d %s", rr.Code, rr.Body)
	}
	demo := decode(t, rr)["data"].(map[string]any)["demo"].(map[string]any)
	if demo["enabled"] != false {
		t.Errorf("enabled = %v, want false", demo["enabled"])
	}
	if _, ok := demo["token"]; ok {
		t.Error("disabled config leaked a demo token")
	}
	if _, ok := demo["email"]; ok {
		t.Error("disabled config exposed a demo email")
	}
}
