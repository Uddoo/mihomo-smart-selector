package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLookupCacheBudgetCredentialsAndNullableEvidence(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("IPQS-KEY") != "secret" || strings.Contains(r.URL.String(), "secret") || r.URL.Query().Get("ip") != "8.8.8.8" || r.URL.Query().Get("strictness") != "0" {
			t.Error("incorrect or unsafe upstream request")
		}
		fmt.Fprint(w, `{"success":true,"fraud_score":100,"connection_type":"Data Center","vpn":false}`)
	}))
	defer server.Close()
	c := NewClient()
	c.endpoint = server.URL
	now := time.Now().UTC()
	c.now = func() time.Time { return now }
	budget := 0
	reserve := func() bool { budget++; return true }
	v := c.Lookup(context.Background(), "::ffff:8.8.8.8", "secret", reserve)
	if v.Status != "ok" || v.TrustScore == nil || *v.TrustScore != 0 || v.VPN == nil || *v.VPN || v.Tor != nil || v.ExpiresAt.Sub(v.FetchedAt) != CacheTTL {
		t.Fatalf("invalid evidence: %+v", v)
	}
	*v.TrustScore = 91
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := c.Lookup(context.Background(), "8.8.8.8", "secret", reserve)
			if got.TrustScore == nil || *got.TrustScore != 0 {
				t.Error("cache aliased a returned score")
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || budget != 1 {
		t.Fatal("cache hits consumed budget", calls.Load(), budget)
	}
	if got := c.Lookup(context.Background(), "8.8.8.8", "", reserve); got.Status != "not_configured" || got.TrustScore != nil {
		t.Fatal("removed credentials reused evidence", got)
	}
	now = now.Add(CacheTTL)
	c.Lookup(context.Background(), "8.8.8.8", "secret", reserve)
	if calls.Load() != 2 {
		t.Fatal("expired evidence was reused")
	}
	encoded, _ := json.Marshal(v)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("secret leaked")
	}
}

func TestLookupFailureMissingScoreAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name, body, status string
		code               int
		score              *int
	}{
		{"missing", `{"success":true}`, "ok", 200, nil},
		{"invalid-score", `{"success":true,"fraud_score":101}`, "ok", 200, nil},
		{"provider-error", `{"success":false,"message":"secret account details"}`, "error", 200, nil},
		{"malformed", `<html>secret</html>`, "error", 200, nil},
		{"quota", `{}`, "rate_limited", 429, nil},
		{"unauthorized", `secret`, "error", 403, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "7200")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := NewClient()
			c.endpoint = server.URL
			v := c.Lookup(context.Background(), "8.8.8.8", "secret", func() bool { return true })
			if v.Status != tc.status || v.TrustScore != nil || strings.Contains(v.Error, "secret") {
				t.Fatal(v)
			}
			if tc.code == 429 || tc.code == 403 || tc.name == "provider-error" {
				c.Lookup(context.Background(), "1.1.1.1", "secret", func() bool { t.Error("backoff consumed budget"); return true })
				if calls.Load() != 1 {
					t.Fatal("ignored account backoff")
				}
				c.Lookup(context.Background(), "1.1.1.1", "new-secret", func() bool { return true })
				if calls.Load() != 2 {
					t.Fatal("new credentials were blocked")
				}
			}
		})
	}
	start := time.Now().UTC()
	if retryAfter("7200", start).Sub(start) != 2*time.Hour || retryAfter("invalid", start).Sub(start) != time.Hour {
		t.Fatal("invalid Retry-After behavior")
	}
}

func TestLookupDeniedBudgetCancellationAndRedirect(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Redirect(w, r, "/leak", 302) }))
	defer server.Close()
	c := NewClient()
	c.endpoint = server.URL
	if c.Lookup(context.Background(), "8.8.8.8", "secret", func() bool { return false }).Status != "budget" {
		t.Fatal("budget was ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if c.Lookup(ctx, "8.8.8.8", "secret", func() bool { t.Error("cancelled lookup consumed budget"); return true }).Status != "cancelled" {
		t.Fatal("cancellation was ignored")
	}
	if requests.Load() != 0 {
		t.Fatal("unbudgeted request")
	}
	if c.Lookup(context.Background(), "8.8.8.8", "secret", func() bool { return true }).Status != "error" || requests.Load() != 1 {
		t.Fatal("redirect followed")
	}
}
