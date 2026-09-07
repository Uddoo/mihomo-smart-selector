package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

func TestServiceCatalogAndBindingAPIsRespectAuthentication(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP.Listen = "0.0.0.0:8788"
	cfg.HTTP.AllowedCIDRs = []string{"127.0.0.1/32"}
	cfg.HTTP.APIToken = "test-token"
	cfg.Scanner.ProbeProfiles = append(cfg.Scanner.ProbeProfiles, config.ProbeProfile{
		ID: "private", Label: "Private API", TransportScope: "HTTP latency only",
		Probes: []config.Probe{{Name: "health", URL: "https://private.example/health", ExpectedStatus: "200"}},
	})
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
	settingsJSON, err := json.Marshal(cfg.RuntimeSettings())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ method, path, body string }{
		{"GET", "/api/v1/services", ""},
		{"PUT", "/api/v1/bindings", `{"group":"Gone","profile_id":""}`},
		{"GET", "/api/v1/settings", ""},
		{"PUT", "/api/v1/settings", string(settingsJSON)},
	} {
		for _, authorized := range []bool{false, true} {
			request := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Content-Type", "application/json")
			if authorized {
				request.Header.Set("Authorization", "Bearer test-token")
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			expected := http.StatusUnauthorized
			if authorized {
				expected = http.StatusOK
			}
			if response.Code != expected {
				t.Fatalf("%s authorized=%v: %d %s", item.path, authorized, response.Code, response.Body.String())
			}
			if authorized && item.path == "/api/v1/services" {
				var catalog map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(response.Body.String(), "private.example") {
					t.Fatal("private template address leaked")
				}
				if len(catalog["profiles"].([]any)) == 0 {
					t.Fatal("empty profile catalog")
				}
			}
		}
	}
}

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

func TestExposedServerAllowsOnlyConfiguredLANWhenPasswordIsDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{Listen: "0.0.0.0:8788", AllowUnauthenticatedLAN: true, AllowedCIDRs: []string{"192.0.2.0/24"}}
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
	allowed := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	allowed.RemoteAddr = "192.0.2.8:1234"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, allowed)
	if response.Code != http.StatusOK {
		t.Fatalf("allowed LAN status = %d", response.Code)
	}
	blocked := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	blocked.RemoteAddr = "198.51.100.8:1234"
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, blocked)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-LAN status = %d", response.Code)
	}
}
