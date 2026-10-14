package ipintel

import (
	"encoding/json"
	"errors"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSettingsPersistRedactClearRestoreAndRevisions(t *testing.T) {
	base := config.Defaults().IPIntelligence
	base.APIKeyEnv = "MSS_GUI_TEST_KEY"
	t.Setenv(base.APIKeyEnv, "server-private-key")
	path := filepath.Join(t.TempDir(), "private.json")
	s, err := OpenSettings(base, path)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	state, err := s.Save(SettingsUpdate{Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "replace", Key: "browser-private-key"}, {ID: "scamalytics", Enabled: &enabled, KeyAction: "keep", Server: "eu"}}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(state)
	if strings.Contains(string(encoded), "private-key") || !state.Providers[0].KeyConfigured {
		t.Fatal("secret returned by settings")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("credential file is not private")
		}
	}
	opened, err := OpenSettings(base, path)
	if err != nil {
		t.Fatal("restart rejected empty Scamalytics setup", err)
	}
	if key, _ := opened.Effective().Providers()[0].Credentials.APIKey(); key != "browser-private-key" {
		t.Fatal("saved credential did not survive restart")
	}
	if _, err = s.Save(SettingsUpdate{Revision: 0, Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "clear"}}}); !errors.Is(err, ErrSettingsConflict) {
		t.Fatal("stale save accepted", err)
	}
	if _, err = s.Save(SettingsUpdate{Revision: 1, Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "clear"}}}); err != nil {
		t.Fatal(err)
	}
	if key, _ := s.Effective().Providers()[0].Credentials.APIKey(); key != "" {
		t.Fatal("cleared key fell back to environment")
	}
	if _, err = s.Save(SettingsUpdate{Revision: 2, Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "server"}}}); err != nil {
		t.Fatal(err)
	}
	if key, _ := s.Effective().Providers()[0].Credentials.APIKey(); key != "server-private-key" {
		t.Fatal("server credentials did not restore")
	}
	if base.IPQualityScore.RuntimeKey != nil {
		t.Fatal("base config was mutated")
	}
}

func TestSettingsValidateAtomicallyAndKeepSecretOnFailure(t *testing.T) {
	s, err := OpenSettings(config.Defaults().IPIntelligence, filepath.Join(t.TempDir(), "private.json"))
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	for _, u := range []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "replace", Key: "a\nb"}, {ID: "ipqualityscore", Enabled: &enabled, KeyAction: "keep", Key: "unexpected-secret"}, {ID: "scamalytics", Enabled: &enabled, KeyAction: "replace", Key: "secret", Username: "", Server: "eu"}, {ID: "other", Enabled: &enabled, KeyAction: "clear"}} {
		if _, err := s.Save(SettingsUpdate{Providers: []ProviderUpdate{u}}); err == nil || strings.Contains(err.Error(), "unexpected-secret") {
			t.Fatal("invalid credentials accepted", err)
		}
		if s.State().Revision != 0 {
			t.Fatal("invalid update changed settings")
		}
	}
	_, err = s.Save(SettingsUpdate{Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "replace", Key: "one"}, {ID: "ipqualityscore", Enabled: &enabled, KeyAction: "clear"}}})
	if err == nil || s.State().Revision != 0 {
		t.Fatal("duplicate update was not atomic")
	}
	s.path = t.TempDir()
	if _, err := s.Save(SettingsUpdate{Providers: []ProviderUpdate{{ID: "ipqualityscore", Enabled: &enabled, KeyAction: "replace", Key: "secret"}}}); err == nil || s.State().Revision != 0 {
		t.Fatal("failed persistence changed active settings")
	}
}
