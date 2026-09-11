package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
	"gopkg.in/yaml.v3"
)

func TestRuntimeRestartAppliesConnectionAndKeepsOfflineSettingsAccessible(t *testing.T) {
	controller := func(version, secret string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret != "" && r.Header.Get("Authorization") != "Bearer "+secret {
				w.WriteHeader(401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"version": version, "proxies": map[string]any{}, "providers": map[string]any{}})
		}))
	}
	first := controller("first-core", "")
	defer first.Close()
	second := controller("second-core", "runtime-test-secret")
	defer second.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	cfg := config.Defaults()
	cfg.HTTP.Listen = address
	cfg.Mihomo.Controller = first.URL
	cfg.Mihomo.SecretEnv = "MSS_RUNTIME_TEST_SECRET"
	t.Setenv(cfg.Mihomo.SecretEnv, "")
	dir := filepath.Join(t.TempDir(), "配置 with spaces")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.Storage.Path = filepath.Join(dir, "data", "selector.db")
	path := filepath.Join(dir, "config.yaml")
	contents, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runService(ctx, path) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("service workers did not stop")
		}
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	base := "http://" + address + "/api/v1"
	awaitBoot := func(previous string) string {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			response, err := client.Get(base + "/service")
			if err == nil {
				var payload struct {
					InstanceID string `json:"instance_id"`
				}
				_ = json.NewDecoder(response.Body).Decode(&payload)
				response.Body.Close()
				if response.StatusCode == 200 && payload.InstanceID != "" && payload.InstanceID != previous {
					return payload.InstanceID
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("new service generation not observed")
		return ""
	}
	request := func(method, route string, payload any, status int, output any) {
		t.Helper()
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequest(method, base+route, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != status {
			t.Fatalf("%s %s status=%d: %s", method, route, response.StatusCode, body)
		}
		if strings.Contains(string(body), "runtime-test-secret") {
			t.Fatal("credential returned by API")
		}
		if output != nil {
			if err := json.Unmarshal(body, output); err != nil {
				t.Fatal(err)
			}
		}
	}
	boot := awaitBoot("")
	var health struct {
		Version string `json:"mihomo_version"`
	}
	request("GET", "/health", nil, 200, &health)
	if health.Version != "first-core" {
		t.Fatal(health)
	}
	var state connection.State
	request("GET", "/connection", nil, 200, &state)
	request("PUT", "/connection", connection.Update{Revision: state.Revision, Controller: second.URL, RequestTimeoutSeconds: 3, SecretAction: "replace", Secret: "runtime-test-secret"}, 200, &state)
	if !state.RestartRequired {
		t.Fatal("no pending settings")
	}
	request("POST", "/service/restart", map[string]any{"instance_id": boot, "confirm": true}, 202, nil)
	nextBoot := awaitBoot(boot)
	request("GET", "/health", nil, 200, &health)
	if health.Version != "second-core" {
		t.Fatal("saved controller or secret not applied")
	}
	request("GET", "/connection", nil, 200, &state)
	if state.RestartRequired || state.Active.Controller != second.URL || !state.Active.SecretConfigured {
		t.Fatal(state)
	}
	request("POST", "/service/restart", map[string]any{"instance_id": boot, "confirm": true}, 409, nil)
	// A bad YAML edit must not stop the currently working service.
	if err := os.WriteFile(path, []byte("http: [invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	request("POST", "/service/restart", map[string]any{"instance_id": nextBoot, "confirm": true}, 409, nil)
	request("GET", "/health", nil, 200, &health)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	// Recovery is based on service generation, even when the saved Controller is offline.
	second.Close()
	request("POST", "/service/restart", map[string]any{"instance_id": nextBoot, "confirm": true}, 202, nil)
	awaitBoot(nextBoot)
	request("GET", "/health", nil, 503, nil)
	request("GET", "/connection", nil, 200, &state)
	if state.RestartRequired {
		t.Fatal("offline connection did not apply")
	}
}

func TestPortConflictDoesNotOpenDatabaseOrStartBackgroundWork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := config.Defaults()
	cfg.HTTP.Listen = listener.Addr().String()
	cfg.Mihomo.SecretEnv = "MSS_PORT_CONFLICT_SECRET"
	t.Setenv(cfg.Mihomo.SecretEnv, "")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "must-not-exist.db")
	path := filepath.Join(t.TempDir(), "config.yaml")
	data, _ := yaml.Marshal(cfg)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := serveOnce(context.Background(), path, &runtimeStorage{}); err == nil || !strings.Contains(err.Error(), "cannot listen") {
		t.Fatalf("conflict: %v", err)
	}
	if _, err := os.Stat(cfg.Storage.Path); !os.IsNotExist(err) {
		t.Fatalf("database touched despite port conflict: %v", err)
	}
}
