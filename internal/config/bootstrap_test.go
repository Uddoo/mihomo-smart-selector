package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableDefaultsDoNotDependOnWorkingDirectoryOrOverwriteSettings(t *testing.T) {
	root := t.TempDir()
	portable := filepath.Join(root, "便携版 with spaces")
	executable := filepath.Join(portable, "mihomo-smart-selector.exe")
	other := filepath.Join(root, "other-working-directory")
	mac, err := DefaultPath("", filepath.Join(portable, "mihomo-smart-selector"), other, "darwin")
	if err != nil || mac != filepath.Join(portable, "config.yaml") {
		t.Fatalf("macOS portable path=%s err=%v", mac, err)
	}
	path, err := DefaultPath("", executable, other, "windows")
	if err != nil || path != filepath.Join(portable, "config.yaml") {
		t.Fatalf("portable path=%s err=%v", path, err)
	}
	created, err := Initialize(path)
	if err != nil || !created {
		t.Fatalf("bootstrap=%v %v", created, err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Storage.Path != filepath.Join(portable, "data", "selector.db") || cfg.AutoSwitch.Enabled || cfg.EgressVerification.Enabled {
		t.Fatalf("invalid first-run config: %+v %v", cfg, err)
	}
	custom := []byte("http:\n  listen: 127.0.0.1:9999\n")
	if err := os.WriteFile(path, custom, 0600); err != nil {
		t.Fatal(err)
	}
	if created, err := Initialize(path); err != nil || created {
		t.Fatalf("existing settings overwritten: %v %v", created, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(custom) {
		t.Fatal("bootstrap modified existing configuration")
	}
	explicit, err := DefaultPath("custom.yaml", executable, other, "windows")
	if err != nil || explicit != filepath.Join(other, "custom.yaml") {
		t.Fatalf("explicit relative path=%s %v", explicit, err)
	}
	unix, err := DefaultPath("", filepath.Join(portable, "selector"), other, "linux")
	if err != nil || unix != filepath.Join(other, "config.yaml") {
		t.Fatalf("existing Unix default changed: %s %v", unix, err)
	}
}
