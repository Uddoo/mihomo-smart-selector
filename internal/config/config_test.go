package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesStorageAndRejectsExposedListenerWithoutGuardrails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selector.yaml")
	data := `
http:
  listen: 0.0.0.0:8788
mihomo:
  controller: http://127.0.0.1:9090
  secret_env: MIHOMO_SECRET
scanner:
  probes:
    - name: trace
      url: https://chatgpt.com/cdn-cgi/trace
      expected_status: "200"
regions:
  - code: JP
    name: Japan
    aliases: [JP]
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "api_token") {
		t.Fatalf("expected exposed-listener validation error, got %v", err)
	}
}

func TestLoadResolvesRelativeStoragePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selector.yaml")
	data := `
http:
  listen: 127.0.0.1:8788
mihomo:
  controller: http://127.0.0.1:9090
  secret_env: MIHOMO_SECRET
storage:
  path: runtime/history.db
scanner:
  probes:
    - name: trace
      url: https://chatgpt.com/cdn-cgi/trace
      expected_status: "200"
regions:
  - code: JP
    name: Japan
    aliases: [JP]
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(path), "runtime", "history.db")
	if got.Storage.Path != want {
		t.Fatalf("storage path = %q, want %q", got.Storage.Path, want)
	}
}

func TestLoadUsesHTTPTokenEnvironmentVariableForExposedListener(t *testing.T) {
	t.Setenv("MSS_TEST_HTTP_TOKEN", "local-test-token")
	path := filepath.Join(t.TempDir(), "selector.yaml")
	data := `
http:
  listen: 0.0.0.0:8788
  api_token_env: MSS_TEST_HTTP_TOKEN
  allowed_cidrs: [192.168.50.0/24]
mihomo:
  controller: http://127.0.0.1:9090
  secret_env: MIHOMO_SECRET
scanner:
  probes:
    - name: trace
      url: https://chatgpt.com/cdn-cgi/trace
      expected_status: "200"
regions:
  - code: JP
    name: Japan
    aliases: [JP]
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTP.APIToken != "local-test-token" {
		t.Fatalf("resolved API token = %q", got.HTTP.APIToken)
	}
}
