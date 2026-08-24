package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
	"github.com/yw-li/mihomo-smart-selector/internal/history"
	"github.com/yw-li/mihomo-smart-selector/internal/mihomo"
	"github.com/yw-li/mihomo-smart-selector/internal/scan"
)

type testController struct{}

func (testController) ListProxies(context.Context) (map[string]mihomo.Proxy, error) {
	return map[string]mihomo.Proxy{}, nil
}
func (testController) ListProviders(context.Context) ([]mihomo.Provider, error) { return nil, nil }
func (testController) Delay(context.Context, string, string, config.Probe, int) (int, error) {
	return 0, nil
}
func (testController) Select(context.Context, string, string) error { return nil }
func (testController) Reachable(context.Context) (string, error)    { return "test-mihomo", nil }

func TestExposedServerRequiresCIDRAndBearerToken(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{
		Listen: "0.0.0.0:8788", APIToken: "test-token", AllowedCIDRs: []string{"192.0.2.0/24"},
	}
	store, err := history.Open(filepath.Join(t.TempDir(), "selector.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := scan.NewManager(cfg, testController{}, store)
	server, err := New(cfg.HTTP, manager, testController{})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	unauthorized.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, unauthorized)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status without token = %d", response.Code)
	}

	staticPage := httptest.NewRequest(http.MethodGet, "/", nil)
	staticPage.RemoteAddr = "192.0.2.10:1234"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, staticPage)
	if response.Code != http.StatusOK {
		t.Fatalf("static page must remain available for token entry, got %d", response.Code)
	}

	authorized := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	authorized.RemoteAddr = "192.0.2.10:1234"
	authorized.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authorized)
	if response.Code != http.StatusOK {
		t.Fatalf("status with token = %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("security header = %q", got)
	}

	loopback := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	loopback.RemoteAddr = "127.0.0.1:1234"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, loopback)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("loopback without token must still be authenticated, got %d", response.Code)
	}
	loopback.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, loopback)
	if response.Code != http.StatusOK {
		t.Fatalf("loopback with token = %d", response.Code)
	}
}
