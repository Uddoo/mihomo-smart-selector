package config

import "testing"

func TestControllerCredentialIdentity(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"http://localhost", "http://LOCALHOST:80/", true},
		{"https://controller.example/base/", "https://CONTROLLER.example:443/base", true},
		{"http://localhost:9090", "http://localhost:9191", false},
		{"https://localhost:9090", "http://localhost:9090", false},
		{"http://localhost:9090/base", "http://localhost:9090/other", false},
		{"http://localhost:9090", "http://127.0.0.1:9090", false},
		{"http://localhost", "http://localhost@other.example", false},
		{"http://localhost", "http://localhost/?x=1", false},
	} {
		if got := SameController(tc.a, tc.b); got != tc.same {
			t.Errorf("SameController(%q, %q)=%v", tc.a, tc.b, got)
		}
	}
}
