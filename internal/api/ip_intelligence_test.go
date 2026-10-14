package api

import (
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/ipintel"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestIPSettingsAPIAuthenticationCSRFAndRedaction(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP.Listen = "0.0.0.0:8788"
	cfg.HTTP.APIToken = "api-token"
	cfg.HTTP.AllowedCIDRs = []string{"127.0.0.1/32"}
	s, err := New(cfg.HTTP, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := ipintel.OpenSettings(cfg.IPIntelligence, filepath.Join(t.TempDir(), "private.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.WithIPIntelligenceSettings(settings)
	body := `{"revision":0,"providers":[{"id":"ipqualityscore","enabled":true,"key_action":"replace","key":"browser-private-key"}]}`
	call := func(method, token, origin, contentType, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:8788/api/v1/ip-intelligence/settings", strings.NewReader(payload))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", contentType)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "browser-private-key") {
			t.Fatal("API returned a saved key")
		}
		return w
	}
	for _, method := range []string{"GET", "PUT"} {
		if w := call(method, "", "", "application/json", body); w.Code != 401 {
			t.Fatal("missing auth accepted", w.Code)
		}
		if w := call(method, "api-token", "https://other.test", "application/json", body); w.Code != 403 {
			t.Fatal("cross origin accepted", w.Code)
		}
	}
	if w := call("PUT", "api-token", "", "text/plain", body); w.Code != 415 {
		t.Fatal(w.Code)
	}
	if w := call("PUT", "api-token", "http://localhost:8788", "application/json", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("GET", "api-token", "", "", ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"key_configured":true`) {
		t.Fatal("bad redacted state", w.Code)
	}
	if w := call("PUT", "api-token", "", "application/json", body); w.Code != 409 {
		t.Fatal("stale browser accepted", w.Code)
	}
}
