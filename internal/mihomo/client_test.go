package mihomo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestSecretFilePrecedenceAndInvalidFiles(t *testing.T) {
	t.Setenv("MSS_TEST_SECRET", "env-secret")
	path := filepath.Join(t.TempDir(), "secret")
	cfg := config.MihomoConfig{Controller: "http://127.0.0.1:9090", SecretEnv: "MSS_TEST_SECRET", SecretFile: path}
	if _, err := New(cfg); err == nil {
		t.Fatal("missing file fell back to environment")
	}
	for _, input := range []string{"", "first\nsecond"} {
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(cfg); err == nil {
			t.Fatal("invalid secret file accepted")
		}
	}
	if err := os.WriteFile(path, []byte("file-secret\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.secret != "file-secret" {
		t.Fatal("file did not override environment or trim newline")
	}
}

func TestClientEscapesNameAndUsesBearerSecret(t *testing.T) {
	t.Setenv("TEST_MIHO_SECRET", "redacted-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer redacted-secret" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.URL.EscapedPath(); got != "/proxies/JP%2F%E6%9D%B1%E4%BA%AC/delay" {
			t.Fatalf("escaped path = %q", got)
		}
		if got := r.URL.Query().Get("expected"); got != "200" {
			t.Fatalf("expected query = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"delay":123}`))
	}))
	defer server.Close()

	client, err := New(config.MihomoConfig{Controller: server.URL, SecretEnv: "TEST_MIHO_SECRET", RequestTimeoutSeconds: 2})
	if err != nil {
		t.Fatal(err)
	}
	delay, err := client.Delay(context.Background(), "JP/東京", "", config.Probe{Name: "trace", URL: "https://chatgpt.com/cdn-cgi/trace", ExpectedStatus: "200"}, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if delay != 123 {
		t.Fatalf("delay = %d", delay)
	}
}

func TestClientUsesProviderHealthcheckForProviderOwnedNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/providers/proxies/provider-A/JP-03/healthcheck" {
			t.Fatalf("escaped path = %q", got)
		}
		if got := r.URL.Query().Get("expected"); got != "" {
			t.Fatalf("provider healthcheck must not send expected-status, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"delay":91}`))
	}))
	defer server.Close()

	client, err := New(config.MihomoConfig{Controller: server.URL, SecretEnv: "UNSET_MIHO_SECRET", RequestTimeoutSeconds: 2})
	if err != nil {
		t.Fatal(err)
	}
	delay, err := client.Delay(context.Background(), "JP-03", "provider-A", config.Probe{Name: "trace", URL: "https://chatgpt.com/cdn-cgi/trace", ExpectedStatus: "200"}, 5000)
	if err != nil || delay != 91 {
		t.Fatalf("provider healthcheck delay = %d, %v", delay, err)
	}
}

func TestClientDoesNotSendAuthorizationWhenSecretIsUnset(t *testing.T) {
	_ = os.Unsetenv("UNSET_MIHO_SECRET")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()

	client, err := New(config.MihomoConfig{Controller: server.URL, SecretEnv: "UNSET_MIHO_SECRET", RequestTimeoutSeconds: 2})
	if err != nil {
		t.Fatal(err)
	}
	version, err := client.Reachable(context.Background())
	if err != nil || version != "test" {
		t.Fatalf("reachable = %q, %v", version, err)
	}
}
