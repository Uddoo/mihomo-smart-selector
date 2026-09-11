package api

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestServiceRestartAuthorizationAdmissionAndIdempotency(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP.Listen = "0.0.0.0:8788"
	cfg.HTTP.APIToken = "api-test-token"
	cfg.HTTP.AllowedCIDRs = []string{"127.0.0.1/32"}
	server, err := New(cfg.HTTP, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	busy := true
	server.WithServiceControl("boot-one", "test", func() error {
		if busy {
			return fmt.Errorf("有扫描正在运行，请等待完成或停止扫描后重启服务")
		}
		return nil
	}, func() { calls.Add(1) })
	call := func(path, method, body, token, origin, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://localhost:8788/api/v1/"+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:2222"
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		return w
	}
	payload := "{\"instance_id\":\"boot-one\",\"confirm\":true}"
	for _, route := range []string{"service", "service/restart"} {
		if w := call(route, "POST", payload, "", "", "application/json"); w.Code != 401 {
			t.Fatalf("unguarded %s: %d", route, w.Code)
		}
	}
	for _, tc := range []struct {
		origin, contentType, body string
		code                      int
	}{
		{"https://other.example", "application/json", payload, 403},
		{"", "text/plain", payload, 415},
		{"", "application/json", "{\"instance_id\":\"boot-one\",\"confirm\":false}", 400},
		{"", "application/json", "{\"instance_id\":\"old-boot\",\"confirm\":true}", 409},
		{"", "application/json", payload, 409},
	} {
		if w := call("service/restart", "POST", tc.body, cfg.HTTP.APIToken, tc.origin, tc.contentType); w.Code != tc.code {
			t.Fatalf("status=%d wanted=%d body=%s", w.Code, tc.code, w.Body)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected request restarted the service")
	}
	busy = false
	for i := 0; i < 2; i++ {
		if w := call("service/restart", "POST", payload, cfg.HTTP.APIToken, "http://localhost:8788", "application/json"); w.Code != 202 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate restart dispatched")
	}
	if w := call("health", "GET", "", cfg.HTTP.APIToken, "", ""); w.Code != 503 {
		t.Fatal("new API work admitted during restart")
	}
	if w := call("service", "GET", "", cfg.HTTP.APIToken, "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "restarting") {
		t.Fatal("restart status unavailable")
	}
}
