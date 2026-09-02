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

func TestResolveProbeProfileNormalizesOpenClashSelectorDecorations(t *testing.T) {
	cfg := Defaults()
	profile, err := cfg.ResolveProbeProfile("🤖 ChatGPT")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "chatgpt" || len(profile.Probes) != 1 || len(profile.StrictProbes) != 1 {
		t.Fatalf("profile = %#v", profile)
	}
	emby, err := cfg.ResolveProbeProfile("🎥 Emby")
	if err != nil {
		t.Fatal(err)
	}
	if !emby.RequiresConfiguration || emby.SetupHint == "" {
		t.Fatalf("Emby profile must require an explicitly configured private endpoint: %#v", emby)
	}
}

func TestValidateRejectsEnabledStrictVerificationWithoutIsolatedRoute(t *testing.T) {
	cfg := Defaults()
	cfg.Scanner.StrictVerification.Enabled = true
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "selector_group") {
		t.Fatalf("expected strict verification route validation error, got %v", err)
	}
}

func TestProbeProfileOverrideConfiguresPrivateEmbyWithoutReplacingDefaults(t *testing.T) {
	cfg := Defaults()
	disabled := false
	cfg.Scanner.ProbeProfileOverrides = map[string]ProbeProfileOverride{
		"emby": {
			Probes:                []Probe{{Name: "emby-public-info", URL: "https://media.example.test/emby/System/Info/Public", ExpectedStatus: "200"}},
			RequiresConfiguration: &disabled,
			SetupHint:             "",
		},
	}
	if err := cfg.applyProbeProfileOverrides(); err != nil {
		t.Fatal(err)
	}
	emby, err := cfg.ResolveProbeProfile("🎥 Emby")
	if err != nil {
		t.Fatal(err)
	}
	if emby.RequiresConfiguration || len(emby.Probes) != 1 || emby.Probes[0].URL != "https://media.example.test/emby/System/Info/Public" {
		t.Fatalf("Emby override = %#v", emby)
	}
	if _, err := cfg.ResolveProbeProfile("🤖 ChatGPT"); err != nil {
		t.Fatalf("other built-in profiles must remain available: %v", err)
	}
}
