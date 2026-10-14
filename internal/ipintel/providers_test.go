package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func providerFixture(id, ip string) string {
	switch id {
	case "ipqualityscore":
		return `{"success":true,"fraud_score":100,"connection_type":"Data Center","vpn":false,"proxy":null}`
	case "scamalytics":
		return fmt.Sprintf(`{"scamalytics":{"status":"ok","mode":"live","ip":%q,"scamalytics_score":20,"scamalytics_isp":"PREMIUM FIELD - upgrade to view","scamalytics_org":"PREMIUM FIELD - upgrade to view","scamalytics_proxy":{"is_datacenter":false,"is_vpn":true}},"external_datasources":{"maxmind_geolite2":{"asn":"13335","as_name":"Example org","ip_country_code":"US"},"ip2proxy_lite":{"proxy_type":"PUB","isp_name":"Example ISP"},"x4bnet":{"is_tor":false}}}`, ip)
	case "ipapi_is":
		return fmt.Sprintf(`{"ip":%q,"is_datacenter":false,"is_vpn":false,"is_abuser":true,"company":{"name":"Example ISP","type":"isp","abuser_score":"0.123 (High)"},"asn":{"asn":13335,"org":"Example org","abuser_score":"0.020 (Low)"},"location":{"country_code":"US","city":"Example city"}}`, ip)
	case "abuseipdb":
		return fmt.Sprintf(`{"data":{"ipAddress":%q,"isPublic":true,"abuseConfidenceScore":0,"totalReports":0,"isTor":false,"countryCode":"US","usageType":"Mobile ISP","isp":"Example ISP"}}`, ip)
	}
	return `{}`
}

func configuredProviders(t *testing.T) config.IPIntelligenceConfig {
	t.Helper()
	cfg := config.Defaults().IPIntelligence
	cfg.APIKeyEnv = "MSS_TEST_MULTI_QS"
	cfg.APIKeyFile = ""
	cfg.Scamalytics.APIKeyEnv = "MSS_TEST_MULTI_SCAM"
	cfg.Scamalytics.Username = "example_account"
	cfg.Scamalytics.UsernameEnv = ""
	cfg.IPAPIIS.APIKeyEnv = "MSS_TEST_MULTI_IPAPI"
	cfg.AbuseIPDB.APIKeyEnv = "MSS_TEST_MULTI_ABUSE"
	for _, p := range cfg.Providers() {
		t.Setenv(p.Credentials.APIKeyEnv, "private-account-token")
	}
	return cfg
}

func pointService(s *Service, base string) {
	for id, c := range s.clients {
		c.endpoint = base + "/" + id + "/"
	}
}

func TestProviderRequestContractsAndDistinctScoreSemantics(t *testing.T) {
	for _, id := range []string{"ipqualityscore", "scamalytics", "ipapi_is", "abuseipdb"} {
		t.Run(id, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "application/json" {
					t.Error("missing JSON contract")
				}
				switch id {
				case "ipqualityscore":
					if r.Method != "GET" || r.Header.Get("IPQS-KEY") != "secret" || r.URL.Query().Get("ip") != "2606:4700:4700::1111" || r.URL.Query().Has("key") {
						t.Error("IPQS request contract")
					}
				case "scamalytics":
					if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/account_name") || r.URL.Query().Get("key") != "secret" || r.URL.Query().Get("ip") != "2606:4700:4700::1111" || r.URL.Query().Has("test") {
						t.Error("Scamalytics request contract")
					}
				case "ipapi_is":
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body)
					if r.Method != "POST" || r.URL.Query().Has("key") || body["key"] != "secret" || body["q"] != "2606:4700:4700::1111" || len(body) != 2 {
						t.Error("ipapi.is request contract")
					}
				case "abuseipdb":
					if r.Method != "GET" || r.Header.Get("Key") != "secret" || r.URL.Query().Get("ipAddress") != "2606:4700:4700::1111" || r.URL.Query().Get("maxAgeInDays") != "30" || r.URL.Query().Has("verbose") {
						t.Error("AbuseIPDB read-only check contract")
					}
				}
				fmt.Fprint(w, providerFixture(id, "2606:4700:4700::1111"))
			}))
			defer server.Close()
			c := newProvider(id)
			c.endpoint = server.URL + "/"
			v := c.lookup(context.Background(), "2606:4700:4700::1111", credentials{key: "secret", username: "account_name", server: "eu"}, func() bool { return true })
			if v.Status != "ok" || v.Provider != id || v.ExpiresAt.Sub(v.FetchedAt) != CacheTTL {
				t.Fatal(v)
			}
			switch id {
			case "ipqualityscore":
				if v.TrustScore == nil || *v.TrustScore != 0 || v.Proxy != nil {
					t.Fatal("zero score or unknown flag lost", v)
				}
			case "scamalytics":
				if v.TrustScore == nil || *v.TrustScore != 80 || v.ISP != "Example ISP" || v.Organization != "Example org" || v.ConnectionType != "" {
					t.Fatal("premium placeholder or false residential classification", v)
				}
			case "ipapi_is":
				if v.TrustScore != nil || v.FraudScore != nil || v.CompanyAbuseScore != "0.123 (High)" || v.Abuser == nil || !*v.Abuser || v.Tor != nil || v.ConnectionType != "ISP" {
					t.Fatal("network abuse turned into composite score", v)
				}
			case "abuseipdb":
				if v.TrustScore != nil || v.FraudScore != nil || v.AbuseConfidenceScore == nil || *v.AbuseConfidenceScore != 0 || v.TotalReports == nil || *v.TotalReports != 0 || v.VPN != nil || v.RecentAbuse != nil {
					t.Fatal("report confidence turned into fraud score", v)
				}
			}
		})
	}
}

func TestProvidersRunConcurrentlyAndCacheIndependently(t *testing.T) {
	cfg := configuredProviders(t)
	s := NewService()
	started := make(chan string, 4)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.Split(r.URL.Path, "/")[1]
		calls.Add(1)
		started <- id
		<-release
		fmt.Fprint(w, providerFixture(id, "8.8.8.8"))
	}))
	defer server.Close()
	pointService(s, server.URL)
	var budget atomic.Int32
	done := make(chan []model.IPIntelligence, 1)
	go func() {
		done <- s.LookupAll(context.Background(), "8.8.8.8", cfg, func() bool { budget.Add(1); return true })
	}()
	for range 4 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			unblock()
			t.Fatal("providers did not start independently")
		}
	}
	unblock()
	values := <-done
	if len(values) != 4 || calls.Load() != 4 || budget.Load() != 4 {
		t.Fatal("provider fanout", len(values), calls.Load(), budget.Load())
	}
	for i, id := range []string{"ipqualityscore", "scamalytics", "ipapi_is", "abuseipdb"} {
		if values[i].Provider != id || values[i].Status != "ok" {
			t.Fatal("unstable source order", values)
		}
	}
	*values[0].TrustScore = 88
	copy := s.LookupAll(context.Background(), "::ffff:8.8.8.8", cfg, func() bool { t.Error("cache hit reserved another request"); return false })
	if calls.Load() != 4 || *copy[0].TrustScore != 0 {
		t.Fatal("shared-IP cache was not isolated")
	}
}

func TestProviderQuotaFailureAndCredentialsDoNotBlockOtherSources(t *testing.T) {
	cfg := configuredProviders(t)
	s := NewService()
	var counts sync.Map
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.Split(r.URL.Path, "/")[1]
		counter, _ := counts.LoadOrStore(id, &atomic.Int32{})
		counter.(*atomic.Int32).Add(1)
		if id == "ipqualityscore" {
			w.Header().Set("Retry-After", "7200")
			w.WriteHeader(429)
			fmt.Fprint(w, `private-account-token`)
			return
		}
		ip := r.URL.Query().Get("ip")
		if id == "abuseipdb" {
			ip = r.URL.Query().Get("ipAddress")
		}
		if id == "ipapi_is" {
			var data map[string]string
			json.NewDecoder(r.Body).Decode(&data)
			ip = data["q"]
		}
		fmt.Fprint(w, providerFixture(id, ip))
	}))
	defer server.Close()
	pointService(s, server.URL)
	reserve := func() bool { return true }
	first := s.LookupAll(context.Background(), "8.8.8.8", cfg, reserve)
	if first[0].Status != "rate_limited" {
		t.Fatal(first)
	}
	second := s.LookupAll(context.Background(), "1.1.1.1", cfg, reserve)
	if second[0].Status != "cooldown" {
		t.Fatal("provider backoff missing", second)
	}
	for _, value := range second[1:] {
		if value.Status != "ok" {
			t.Fatal("one provider blocked peers", second)
		}
	}
	counter, _ := counts.Load("ipqualityscore")
	if counter.(*atomic.Int32).Load() != 1 {
		t.Fatal("quota source retried early")
	}
	t.Setenv("MSS_TEST_MULTI_QS", "")
	disabled := false
	cfg.AbuseIPDB.Enabled = &disabled
	cfg.AbuseIPDB.APIKeyFile = "does-not-exist"
	third := s.LookupAll(context.Background(), "1.1.1.1", cfg, func() bool { t.Error("cached or disabled source queried"); return false })
	if third[0].Status != "not_configured" || third[3].Status != "disabled" || third[1].Status != "ok" || third[2].Status != "ok" {
		t.Fatal(third)
	}
	encoded, _ := json.Marshal(third)
	if strings.Contains(string(encoded), "private-account-token") {
		t.Fatal("provider error exposed a credential")
	}
}

func TestAdaptersRejectForeignIPTestModeAndMalformedEvidence(t *testing.T) {
	for _, id := range []string{"scamalytics", "ipapi_is", "abuseipdb"} {
		c := newProvider(id)
		if _, err := c.decode([]byte(providerFixture(id, "1.1.1.1")), "8.8.8.8"); err == nil {
			t.Fatal("foreign IP accepted", id)
		}
		if _, err := c.decode([]byte(`{"private":"secret"}`), "8.8.8.8"); err == nil {
			t.Fatal("missing evidence accepted", id)
		}
	}
	c := newProvider("scamalytics")
	if _, err := c.decode([]byte(strings.Replace(providerFixture("scamalytics", "8.8.8.8"), `"live"`, `"test"`, 1)), "8.8.8.8"); err == nil {
		t.Fatal("test score entered live cache")
	}
	if _, err := c.decode([]byte(`{"scamalytics":{"status":"error","error":"secret","credits":{"remaining":0}}}`), "8.8.8.8"); err != errQuota {
		t.Fatal("Scamalytics credits not recognized", err)
	}
	if _, err := newProvider("ipapi_is").decode([]byte(`{"error":"secret","error_code":"ERR_QUOTA_EXCEEDED"}`), "8.8.8.8"); err != errQuota {
		t.Fatal("ipapi.is quota not recognized", err)
	}
	v, err := decodeAbuseIPDB([]byte(`{"data":{"ipAddress":"8.8.8.8","abuseConfidenceScore":101,"totalReports":-1}}`), "8.8.8.8")
	if err != nil || v.AbuseConfidenceScore != nil || v.TotalReports != nil || v.VPN != nil {
		t.Fatal("invalid AbuseIPDB score accepted", v, err)
	}
}

func TestScamalyticsNodeAccountIdentityAndSanitizedTransportErrors(t *testing.T) {
	c := newProvider("scamalytics")
	req, err := c.request(context.Background(), "8.8.8.8", credentials{key: "secret", username: "account", server: "us"})
	if err != nil || req.URL.Host != "api11.scamalytics.com" {
		t.Fatal("wrong account node", err)
	}
	c.http.Transport = errorTransport(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unsafe transport error: %s", r.URL.String())
	})
	v := c.lookup(context.Background(), "8.8.8.8", credentials{key: "private-account-token", username: "account", server: "eu"}, func() bool { return true })
	encoded, _ := json.Marshal(v)
	if v.Status != "error" || strings.Contains(string(encoded), "private-account-token") || strings.Contains(string(encoded), "scamalytics.com/v3") {
		t.Fatal("raw URL error leaked", string(encoded))
	}
	if v := c.lookup(context.Background(), "8.8.8.8", credentials{key: "secret", username: "../other", server: "eu"}, func() bool { t.Error("invalid account used budget"); return true }); v.Status != "not_configured" {
		t.Fatal(v)
	}
	// Same key on a different account must not reuse that account's score.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, providerFixture("scamalytics", "8.8.8.8"))
	}))
	defer server.Close()
	c = newProvider("scamalytics")
	c.endpoint = server.URL
	var used int
	for _, account := range []credentials{{key: "same", username: "one", server: "eu"}, {key: "same", username: "two", server: "eu"}, {key: "same", username: "two", server: "us"}} {
		c.lookup(context.Background(), "8.8.8.8", account, func() bool { used++; return true })
	}
	if used != 3 {
		t.Fatal("accounts shared cached evidence", used)
	}
}

type errorTransport func(*http.Request) (*http.Response, error)

func TestProviderBudgetCancellationAndCredentialFileFailure(t *testing.T) {
	cfg := configuredProviders(t)
	s := NewService()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.Split(r.URL.Path, "/")[1]
		calls.Add(1)
		fmt.Fprint(w, providerFixture(id, "8.8.8.8"))
	}))
	defer server.Close()
	pointService(s, server.URL)
	for _, v := range s.LookupAll(context.Background(), "8.8.8.8", cfg, func() bool { return false }) {
		if v.Status != "budget" {
			t.Fatal("provider bypassed shared budget", v)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, v := range s.LookupAll(ctx, "8.8.8.8", cfg, func() bool { t.Error("cancelled provider took budget"); return true }) {
		if v.Status != "cancelled" {
			t.Fatal("provider ignored cancellation", v)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("cancelled or unbudgeted network request")
	}
	cfg.IPAPIIS.APIKeyEnv = ""
	cfg.IPAPIIS.APIKeyFile = "nonexistent-private-key-path"
	values := s.LookupAll(context.Background(), "8.8.8.8", cfg, func() bool { return true })
	if values[2].Status != "error" || strings.Contains(values[2].Error, "private-key-path") || calls.Load() != 3 {
		t.Fatal("credential file failure was not isolated", values)
	}
	for _, i := range []int{0, 1, 3} {
		if values[i].Status != "ok" {
			t.Fatal("credential file failure blocked another source", values)
		}
	}
}

func (f errorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
