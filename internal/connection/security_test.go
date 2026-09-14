package connection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExistingSecretsCannotMoveToAnotherController(t *testing.T) {
	for _, source := range []string{"server", "custom"} {
		t.Run(source, func(t *testing.T) {
			m, _, path := setup(t)
			if source == "custom" {
				u := update(m)
				u.SecretAction, u.Secret = "replace", "saved-test-secret"
				if _, err := m.Save(u); err != nil {
					t.Fatal(err)
				}
			}
			before := m.State()
			beforeFile, _ := os.ReadFile(path)
			var requests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Write([]byte(`{"version":"must-not-be-called"}`))
			}))
			defer target.Close()
			for _, address := range []string{target.URL, "https://127.0.0.1:9090", m.base.Controller + "/other"} {
				for _, action := range []string{"keep", "server"} {
					u := update(m)
					u.Controller, u.SecretAction = address, action
					if _, err := m.Test(context.Background(), u); err == nil {
						t.Fatalf("test accepted %s/%s", address, action)
					}
					if _, err := m.Save(u); err == nil {
						t.Fatalf("save accepted %s/%s", address, action)
					}
				}
			}
			if requests.Load() != 0 || m.State() != before {
				t.Fatal("rejected updates sent a request or changed state")
			}
			afterFile, _ := os.ReadFile(path)
			if string(afterFile) != string(beforeFile) {
				t.Fatal("rejected updates changed saved credentials")
			}
		})
	}
}

func TestNewTargetRequiresExplicitCredentialsAndRetainsItsOwnSecret(t *testing.T) {
	m, base, path := setup(t)
	var authorization atomic.Value
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization.Store(r.Header.Get("Authorization"))
		w.Write([]byte(`{"version":"test"}`))
	}))
	defer target.Close()
	for _, action := range []string{"replace", "none"} {
		u := update(m)
		u.Controller, u.SecretAction = target.URL, action
		want := ""
		if action == "replace" {
			u.Secret = "new-target-test-secret"
			want = "Bearer " + u.Secret
		}
		if _, err := m.Test(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		if authorization.Load() != want {
			t.Fatal("unexpected credential sent to new target")
		}
		if _, err := m.Save(u); err != nil {
			t.Fatal(err)
		}
		// A saved custom credential stays usable at its own endpoint after restart.
		restarted, _, client, err := Open(base, path)
		if err != nil {
			t.Fatal(err)
		}
		defer client.CloseIdleConnections()
		if _, err := restarted.Test(context.Background(), update(restarted)); err != nil || authorization.Load() != want {
			t.Fatal("same-target credential reuse failed", err)
		}
		// Clearing a custom credential cannot authorize the YAML credential here.
		u = update(m)
		u.SecretAction = "server"
		if _, err := m.Save(u); err == nil {
			t.Fatal("saved target laundered the server credential")
		}
	}
}

func TestSavedOverrideCannotAuthorizeServerSecretOrPublicTarget(t *testing.T) {
	_, base, _ := setup(t)
	for _, r := range []record{
		{Override: true, Controller: "http://127.0.0.1:9191", RequestTimeoutSeconds: 2, SecretSource: "server"},
		{Override: true, Controller: "http://203.0.113.5:9090", RequestTimeoutSeconds: 2, SecretSource: "none"},
	} {
		path := filepath.Join(t.TempDir(), "connection.json")
		data, _ := json.Marshal(r)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Open(base, path); err == nil || strings.Contains(err.Error(), "test-secret") {
			t.Fatal("unsafe saved override accepted or leaked a credential", err)
		}
	}
}
