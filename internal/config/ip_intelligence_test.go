package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIPIntelligenceCredentialSourcesAndRuntimeRedaction(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "key")
	os.WriteFile(secretPath, []byte("file-secret\n"), 0600)
	t.Setenv("MSS_IPQS_TEST_KEY", "environment-secret")
	c := IPIntelligenceConfig{APIKeyEnv: "MSS_IPQS_TEST_KEY", APIKeyFile: secretPath}
	if key, err := c.APIKey(); err != nil || key != "environment-secret" {
		t.Fatal(key, err)
	}
	t.Setenv("MSS_IPQS_TEST_KEY", "")
	if key, err := c.APIKey(); err != nil || key != "file-secret" {
		t.Fatal(key, err)
	}
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("ip_intelligence:\n  api_key_file: key\n"), 0600)
	cfg, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IPIntelligence.APIKeyFile != secretPath {
		t.Fatal("relative credential path", cfg.IPIntelligence.APIKeyFile)
	}
	encoded, _ := json.Marshal(cfg.RuntimeSettings())
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "ip_intelligence") {
		t.Fatal("credentials exposed in runtime settings")
	}
	c.APIKeyFile = filepath.Join(dir, "missing")
	if _, err = c.APIKey(); err == nil || strings.Contains(err.Error(), dir) {
		t.Fatal("credential error was not sanitized", err)
	}
}

func TestIPIntelligenceProviderFilesLegacyAndValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `ip_intelligence:
  api_key_env: LEGACY_KEY
  api_key_file: legacy.key
  ipqualityscore:
    api_key_file: ipqs.key
  scamalytics:
    api_key_file: scam.key
    username: account_name
    server: us
  ipapi_is:
    api_key_file: ipapi.key
  abuseipdb:
    enabled: false
    api_key_file: abuse.key
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	providers := cfg.IPIntelligence.Providers()
	for i, name := range []string{"ipqs.key", "scam.key", "ipapi.key", "abuse.key"} {
		if providers[i].Credentials.APIKeyFile != filepath.Join(dir, name) {
			t.Fatal("relative provider secret path", providers[i])
		}
	}
	if providers[0].Credentials.APIKeyEnv != "" || providers[1].Username != "account_name" || providers[1].Server != "us" || providers[3].Credentials.Enabled == nil || *providers[3].Credentials.Enabled {
		t.Fatal("provider config or legacy precedence incorrect", providers)
	}
	legacy := IPIntelligenceConfig{APIKeyEnv: "LEGACY_KEY", APIKeyFile: "legacy.key"}.Providers()[0]
	if legacy.Credentials.APIKeyEnv != "LEGACY_KEY" || legacy.Credentials.APIKeyFile != "legacy.key" {
		t.Fatal("legacy config no longer works")
	}
	for _, field := range []string{"server", "username"} {
		bad := cfg.IPIntelligence
		if field == "server" {
			bad.Scamalytics.Server = "private-value"
		} else {
			bad.Scamalytics.Username = "private-value/other"
		}
		if err := bad.Validate(); err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatal("unsafe provider validation", err)
		}
	}
	encoded, _ := json.Marshal(cfg.RuntimeSettings())
	if strings.Contains(string(encoded), "account_name") || strings.Contains(string(encoded), "scam.key") || strings.Contains(string(encoded), "LEGACY_KEY") {
		t.Fatal("account config exposed by settings")
	}
}
