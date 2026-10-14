package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEgressControllerIsBackendOnlyAndLoopback(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:9091", "http://[::1]:9091", "http://192.168.50.2:9091", "https://example.com", "http://user:key@127.0.0.1:9091"} {
		cfg := Defaults()
		cfg.EgressController = &MihomoConfig{Controller: endpoint, RequestTimeoutSeconds: 8}
		valid := endpoint == "http://127.0.0.1:9091" || endpoint == "http://[::1]:9091"
		if (cfg.Validate() == nil) != valid {
			t.Fatal(endpoint)
		}
		if valid {
			next, err := cfg.WithRuntimeSettings(cfg.RuntimeSettings())
			if err != nil || next.EgressController.Controller != endpoint {
				t.Fatal("runtime save discarded isolated controller", err)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("egress_controller:\n  controller: http://127.0.0.1:9091\n  secret_file: probe-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EgressController.SecretFile != filepath.Join(filepath.Dir(path), "probe-secret") || cfg.EgressController.RequestTimeoutSeconds != cfg.Mihomo.RequestTimeoutSeconds {
		t.Fatal("probe defaults or credential path not resolved")
	}
}
