package mihomo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
)

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
