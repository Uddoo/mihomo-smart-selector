package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
)

func TestConnectionAPIAuthenticationValidationAndRedaction(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP.Listen = "0.0.0.0:8788"
	cfg.HTTP.APIToken = "test-api-token"
	cfg.HTTP.AllowedCIDRs = []string{"127.0.0.1/32"}
	cfg.Mihomo.SecretEnv = "MSS_CONNECTION_API_SECRET"
	t.Setenv(cfg.Mihomo.SecretEnv, "server-test-secret")
	m, _, _, err := connection.Open(cfg.Mihomo, filepath.Join(t.TempDir(), "connection.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg.HTTP, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.WithConnection(m)
	payload := `{"revision":0,"controller":"http://localhost:9191","request_timeout_seconds":8,"secret_action":"replace","secret":"draft-test-secret"}`
	call := func(method, route, body, token, origin, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://localhost:8788/api/v1/"+route, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:4567"
		r.Header.Set("Content-Type", contentType)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "test-secret") {
			t.Fatal("API exposed a secret")
		}
		return w
	}
	for _, item := range []struct{ method, route string }{{"GET", "connection"}, {"PUT", "connection"}, {"POST", "connection/test"}} {
		if got := call(item.method, item.route, payload, "", "", "application/json"); got.Code != 401 {
			t.Fatalf("unprotected route %s: %d", item.route, got.Code)
		}
		if got := call(item.method, item.route, payload, cfg.HTTP.APIToken, "https://other.example", "application/json"); got.Code != 403 {
			t.Fatalf("cross-origin request allowed: %d", got.Code)
		}
	}
	if got := call("POST", "connection/test", payload, cfg.HTTP.APIToken, "", "text/plain"); got.Code != 415 {
		t.Fatal("plain-text credential request allowed")
	}
	if got := call("PUT", "connection", `{bad-json`, cfg.HTTP.APIToken, "", "application/json"); got.Code != 400 {
		t.Fatal("bad JSON accepted")
	}
	w := call("PUT", "connection", payload, cfg.HTTP.APIToken, "http://localhost:8788", "application/json")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var state connection.State
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if !state.RestartRequired || !state.Saved.SecretConfigured || state.Active.Controller != cfg.Mihomo.Controller {
		t.Fatal("wrong pending state")
	}
	w = call("GET", "connection", "", cfg.HTTP.APIToken, "", "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("state not available or cacheable")
	}
	w = call("PUT", "connection", payload, cfg.HTTP.APIToken, "", "application/json")
	if w.Code != 409 {
		t.Fatal("stale update accepted")
	}
}

func TestConnectionAPIWorksWhileControllerIsUnavailable(t *testing.T) {
	cfg := config.Defaults()
	cfg.Mihomo.SecretEnv = "MSS_CONNECTION_OFFLINE_SECRET"
	t.Setenv(cfg.Mihomo.SecretEnv, "")
	m, _, _, err := connection.Open(cfg.Mihomo, filepath.Join(t.TempDir(), "connection.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg.HTTP, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.WithConnection(m)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/connection", nil))
	if w.Code != 200 {
		t.Fatal("settings depend on a live controller")
	}
}
