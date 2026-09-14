package mihomo

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestConnectionTargetPolicy(t *testing.T) {
	for _, tc := range []struct {
		target string
		allow  bool
	}{
		{"http://127.0.0.1:9090", true}, {"http://192.168.1.1:9090", true},
		{"http://10.0.0.2:9090", true}, {"http://172.16.1.1:9090", true},
		{"http://[::1]:9090", true}, {"http://[fd12::1]:9090", true},
		{"http://[::ffff:192.168.1.1]:9090", true},
		{"http://169.254.169.254", false}, {"http://100.100.100.200", false},
		{"http://[fd00:ec2::254]", false}, {"http://[fe80::1%25eth0]", false},
		{"http://203.0.113.5", false}, {"http://[::ffff:203.0.113.5]", false},
		{"http://0.0.0.0", false}, {"http://[::]", false}, {"http://224.0.0.1", false},
		{"ftp://127.0.0.1", false}, {"http://user:secret@127.0.0.1", false},
		{"http://127.0.0.1/?x=1", false}, {"http://127.0.0.1/#other", false},
	} {
		t.Run(tc.target, func(t *testing.T) {
			err := ValidateConnectionTarget(tc.target, "http://127.0.0.1:9090")
			if (err == nil) != tc.allow {
				t.Fatalf("allow=%v, err=%v", tc.allow, err)
			}
		})
	}
	public := "https://203.0.113.5:9090/base"
	if err := ValidateConnectionTarget(public, public); err != nil {
		t.Fatal("operator-configured remote Controller was rejected", err)
	}
	if err := ValidateConnectionTarget(public+"/other", public); err == nil {
		t.Fatal("trusted URL allowed an unrelated base path")
	}
}

func TestLocalURLPreflight(t *testing.T) {
	for _, tc := range []struct {
		target string
		allow  bool
	}{
		{"http://localhost:9090", true},
		{"http://127.0.0.1:9090", true},
		{"http://192.168.1.1:9090/base", true},
		{"http://[fd12::1]:9090", true},
		{"http://169.254.169.254", false},
		{"http://[fd00:ec2::254]", false},
		{"http://203.0.113.5", false},
		{"http://[::ffff:203.0.113.5]", false},
		{"http://127.0.0.1/?token=value", false},
		{"file:///etc/passwd", false},
	} {
		if got := IsLocalURL(context.Background(), tc.target); got != tc.allow {
			t.Errorf("IsLocalURL(%q)=%v", tc.target, got)
		}
	}
}

func TestControllerDialChecksAllDNSAnswersAndPinsTheSelectedIP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers []string
		trusted bool
		allow   bool
	}{
		{"private", []string{"192.168.1.1", "fd12::1"}, false, true},
		{"mixed", []string{"127.0.0.1", "169.254.169.254"}, false, false},
		{"public", []string{"203.0.113.5"}, false, false},
		{"metadata", []string{"fd00:ec2::254"}, false, false},
		{"configured public", []string{"203.0.113.5"}, true, true},
		{"empty", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookups, dials := 0, 0
			policy := controllerDialer{trusted: tc.trusted,
				lookup: func(context.Context, string, string) ([]netip.Addr, error) {
					lookups++
					if lookups > 1 {
						return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
					}
					var result []netip.Addr
					for _, address := range tc.answers {
						result = append(result, netip.MustParseAddr(address))
					}
					return result, nil
				},
				dial: func(_ context.Context, network, address string) (net.Conn, error) {
					dials++
					if address != net.JoinHostPort(tc.answers[0], "9090") || network != "tcp" {
						t.Fatalf("dial did not use the verified IP: %s %s", network, address)
					}
					left, right := net.Pipe()
					right.Close()
					return left, nil
				},
			}
			conn, err := policy.DialContext(context.Background(), "tcp", "controller.example:9090")
			if conn != nil {
				conn.Close()
			}
			if (err == nil) != tc.allow || lookups != 1 || (dials > 0) != tc.allow {
				t.Fatalf("allow=%v err=%v lookups=%d dials=%d", tc.allow, err, lookups, dials)
			}
		})
	}
}

func TestControllerDialRevalidatesDNSOnEveryConnection(t *testing.T) {
	lookups, dials := 0, 0
	policy := controllerDialer{
		lookup: func(context.Context, string, string) ([]netip.Addr, error) {
			lookups++
			if lookups == 1 {
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			dials++
			return nil, errors.New("test connection refused")
		},
	}
	policy.DialContext(context.Background(), "tcp", "controller.example:9090")
	if _, err := policy.DialContext(context.Background(), "tcp", "controller.example:9090"); !errors.Is(err, errControllerTarget) || dials != 1 {
		t.Fatal("DNS rebinding bypassed target validation", err)
	}
}

func TestRestrictedControllerTransportIgnoresProxiesAndRedirects(t *testing.T) {
	requests := 0
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer redirectTarget.Close()
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer controller.Close()
	client, err := NewForConnection(config.MihomoConfig{Controller: controller.URL, RequestTimeoutSeconds: 2}, "test-secret", "http://127.0.0.1:9090")
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	if transport := client.client.Transport.(*http.Transport); transport.Proxy != nil || transport.DialContext == nil {
		t.Fatal("transport can bypass the dial policy")
	}
	if _, err := client.Reachable(context.Background()); err == nil || requests != 0 {
		t.Fatal("redirect followed or accepted", err)
	}
}
