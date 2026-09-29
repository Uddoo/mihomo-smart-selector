package scan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
)

func TestClaudeAPIAuthenticationBoundary(t *testing.T) {
	cfg := config.Defaults()
	profile, err := cfg.ProbeProfileByID("claude-api")
	if err != nil || len(profile.StrictProbes) != 1 {
		t.Fatalf("Claude API strict profile: %+v, %v", profile, err)
	}
	store, err := history.Open(t.TempDir() + "/selector.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := NewManager(cfg, &fakeMihomo{}, store)
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"auth boundary", `{"type":"error","error":{"type":"authentication_error","message":"x-api-key header is required"}}`, "passed", 401},
		{"formatted auth error", "{\n  \"error\": {\"type\": \"authentication_error\"}\n}", "passed", 401},
		{"generic unauthorized", "<html>Unauthorized</html>", "failed", 401},
		{"different API error", `{"error":{"type":"permission_error"}}`, "failed", 401},
		{"empty unauthorized", "", "failed", 401},
		{"forbidden", `{"error":{"type":"permission_error"}}`, "restricted", 403},
		{"rate limited", `{"error":{"type":"rate_limit_error"}}`, "failed", 429},
		{"unexpected success", `{"error":{"type":"authentication_error"}}`, "failed", 200},
		{"upstream error", `{"error":{"type":"authentication_error"}}`, "failed", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" || r.Header.Get("Cookie") != "" {
					t.Error("probe must be a credential-free GET")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			probe := profile.StrictProbes[0]
			probe.URL = server.URL
			check := manager.runStrictCheck(context.Background(), server.Client(), probe)
			if check.Status != tc.want || check.ObservedStatus != tc.status {
				t.Fatalf("check=%+v, want %s / HTTP %d", check, tc.want, tc.status)
			}
			if tc.want == "passed" && (check.BodyMatched == nil || !*check.BodyMatched) {
				t.Fatal("authentication response was not checked")
			}
		})
	}
}
