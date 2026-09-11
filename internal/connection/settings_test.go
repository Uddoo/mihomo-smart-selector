package connection

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func setup(t *testing.T) (*Manager, config.MihomoConfig, string) {
	t.Helper()
	cfg := config.Defaults().Mihomo
	cfg.SecretEnv = "MSS_CONNECTION_TEST_SECRET"
	t.Setenv(cfg.SecretEnv, "server-test-secret")
	path := filepath.Join(t.TempDir(), "selector.db.connection.json")
	m, _, _, err := Open(cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	return m, cfg, path
}

func update(m *Manager) Update {
	s := m.State()
	return Update{Revision: s.Revision, Controller: s.Saved.Controller, RequestTimeoutSeconds: s.Saved.RequestTimeoutSeconds, SecretAction: "keep"}
}

func TestSaveRestartKeepClearAndRestore(t *testing.T) {
	m, base, path := setup(t)
	u := update(m)
	u.Controller, u.SecretAction, u.Secret = "http://127.0.0.1:9191/", "replace", "custom-test-secret"
	s, err := m.Save(u)
	if err != nil {
		t.Fatal(err)
	}
	if !s.RestartRequired || s.Active.Controller != base.Controller || s.Saved.Controller != "http://127.0.0.1:9191" || s.Saved.SecretSource != "custom" {
		t.Fatalf("bad pending state: %+v", s)
	}
	public, _ := json.Marshal(s)
	if strings.Contains(string(public), u.Secret) || strings.Contains(string(public), "server-test-secret") {
		t.Fatal("secret leaked in state")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v", info.Mode())
	}
	restarted, effective, _, err := Open(base, path)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.State().RestartRequired || effective.Controller != s.Saved.Controller {
		t.Fatal("override not applied at restart")
	}
	_, secret, _ := restarted.resolve(restarted.saved)
	if secret != u.Secret {
		t.Fatal("secret not persisted")
	}
	keep := update(restarted)
	keep.RequestTimeoutSeconds = 12
	if _, err := restarted.Save(keep); err != nil {
		t.Fatal(err)
	}
	_, secret, _ = restarted.resolve(restarted.saved)
	if secret != u.Secret {
		t.Fatal("keep erased the saved secret")
	}
	clear := update(restarted)
	clear.SecretAction = "none"
	if _, err := restarted.Save(clear); err != nil {
		t.Fatal(err)
	}
	_, secret, _ = restarted.resolve(restarted.saved)
	if secret != "" || restarted.State().Saved.SecretConfigured {
		t.Fatal("clear fell back to the environment")
	}
	server := update(restarted)
	server.SecretAction = "server"
	if _, err := restarted.Save(server); err != nil {
		t.Fatal(err)
	}
	_, secret, _ = restarted.resolve(restarted.saved)
	if secret != "server-test-secret" {
		t.Fatal("server secret was not restored")
	}
	reset, err := restarted.Save(Update{Revision: restarted.State().Revision, UseServer: true})
	if err != nil {
		t.Fatal(err)
	}
	if reset.Override || !reset.RestartRequired || reset.Saved.Controller != base.Controller {
		t.Fatal("bad restore state")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "test-secret") {
		t.Fatal("restoring defaults retained secret bytes")
	}
	restored, effective, _, err := Open(base, path)
	if err != nil || effective != base || restored.State().RestartRequired {
		t.Fatal("restore did not survive restart", err)
	}
}

func TestSecretFilePrecedenceAndNoCopyOfServerSecret(t *testing.T) {
	_, base, path := setup(t)
	base.SecretFile = filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(base.SecretFile, []byte("file-test-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, _, _, err := Open(base, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Save(update(m)); err != nil {
		t.Fatal(err)
	}
	_, secret, err := m.resolve(m.saved)
	if err != nil || secret != "file-test-secret" {
		t.Fatal("secret file priority changed", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "test-secret") || strings.Contains(string(data), base.SecretFile) {
		t.Fatal("server credentials were copied into override")
	}
}

func TestTestReadsVersionWithoutSavingOrApplying(t *testing.T) {
	m, _, path := setup(t)
	var requests atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.URL.Path != "/base/version" || r.Header.Get("Authorization") != "Bearer draft-test-secret" {
			t.Errorf("unexpected Controller request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"test-mihomo"}`))
	}))
	defer controller.Close()
	u := update(m)
	u.Controller, u.SecretAction, u.Secret = controller.URL+"/base", "replace", "draft-test-secret"
	before := m.State()
	version, err := m.Test(context.Background(), u)
	if err != nil || version != "test-mihomo" || requests.Load() != 1 {
		t.Fatal("test failed", err)
	}
	if m.State() != before {
		t.Fatal("test changed connection state")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("test wrote configuration")
	}
}

func TestFailedAndRedirectedTestsDoNotExposeCredentials(t *testing.T) {
	m, _, _ := setup(t)
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, mode := range []string{"auth", "redirect", "invalid", "empty"} {
		t.Run(mode, func(t *testing.T) {
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "auth":
					http.Error(w, "private-test-secret", 401)
				case "redirect":
					http.Redirect(w, r, target.URL, 302)
				case "invalid":
					w.Write([]byte("private-test-secret"))
				case "empty":
					w.Write([]byte(`{"version":""}`))
				}
			}))
			defer controller.Close()
			u := update(m)
			u.Controller, u.SecretAction, u.Secret = controller.URL, "replace", "private-test-secret"
			_, err := m.Test(context.Background(), u)
			if err == nil || strings.Contains(err.Error(), u.Secret) || strings.Contains(err.Error(), controller.URL) {
				t.Fatal("unsafe error", err)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("Controller redirect was followed")
	}
}

func TestInvalidAndStaleUpdatesAndFailedWritesPreserveState(t *testing.T) {
	m, _, path := setup(t)
	for _, address := range []string{"", "localhost:9090", "file:///etc/passwd", "ftp://localhost", "http://user:secret@localhost", "http://localhost/?token=secret", "http://localhost/#part", "http://localhost/?", "http://localhost:bad", "http://localhost:0", "http://localhost:99999", "http://local host"} {
		u := update(m)
		u.Controller = address
		if _, err := m.Save(u); err == nil {
			t.Errorf("accepted %q", address)
		}
	}
	for _, value := range []string{"", "\r\n", "bad\nsecret", "bad\tsecret", strings.Repeat("x", 4097)} {
		u := update(m)
		u.SecretAction, u.Secret = "replace", value
		if _, err := m.Save(u); err == nil {
			t.Error("invalid secret accepted")
		}
	}
	u := update(m)
	if _, err := m.Save(u); err != nil {
		t.Fatal(err)
	}
	before := m.State()
	if _, err := m.Save(u); !errors.Is(err, ErrConflict) {
		t.Fatal("stale save accepted")
	}
	if _, err := m.Test(context.Background(), u); !errors.Is(err, ErrConflict) {
		t.Fatal("stale test accepted")
	}
	data, _ := os.ReadFile(path)
	m.path = t.TempDir() // Rename over a directory must fail on every supported OS.
	u = update(m)
	u.Controller = "https://controller.example"
	if _, err := m.Save(u); err == nil {
		t.Fatal("write should fail")
	}
	if m.State() != before {
		t.Fatal("failed write changed state")
	}
	after, _ := os.ReadFile(path)
	if string(data) != string(after) {
		t.Fatal("failed write changed the old file")
	}
}

func TestCancelledConnectionTest(t *testing.T) {
	m, _, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Test(ctx, update(m)); err == nil {
		t.Fatal("cancelled test succeeded")
	}
}

func TestUnavailableServerSecretCannotReplaceWorkingCustomConfiguration(t *testing.T) {
	m, _, _ := setup(t)
	u := update(m)
	u.SecretAction, u.Secret = "replace", "working-test-secret"
	if _, err := m.Save(u); err != nil {
		t.Fatal(err)
	}
	before := m.State()
	m.base.SecretFile = filepath.Join(t.TempDir(), "missing-secret")
	u = update(m)
	u.SecretAction = "server"
	if _, err := m.Save(u); err == nil {
		t.Fatal("saved unavailable server secret")
	}
	if m.State() != before {
		t.Fatal("failed save replaced working settings")
	}
	if _, err := m.Save(Update{Revision: before.Revision, UseServer: true}); err == nil {
		t.Fatal("restored unavailable server secret")
	}
}

func TestCorruptConnectionFileFailsWithoutLeakingContents(t *testing.T) {
	_, base, path := setup(t)
	for _, data := range []string{"", "private-test-secret", `{"override":true,"secret":"private-test-secret"}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := Open(base, path)
		if err == nil || strings.Contains(err.Error(), "private-test-secret") {
			t.Fatal("corrupt file was accepted or exposed", err)
		}
	}
}
