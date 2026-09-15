package api

import (
	"strings"
	"testing"
)

func TestConnectivitySources(t *testing.T) {
	got, err := connectivitySources([]byte(`[{"url":"https://www.google.com/generate_204"},{"url":"https://github.com/favicon.ico"},{"url":"https://github.com/other"}]`))
	if err != nil || got != "https://github.com https://www.google.com" {
		t.Fatalf("sources=%q err=%v", got, err)
	}
	for _, value := range []string{`http://example.com`, `https://user:secret@example.com`, `https://*.example.com`, `https://example.com:443`, `https://127.0.0.1`, `https://localhost`, `https://example.com;script-src`} {
		if _, err := connectivitySources([]byte(`[{"url":"` + value + `"}]`)); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestEmbeddedConnectivityCatalog(t *testing.T) {
	data, err := embeddedStatic.ReadFile("static/connectivity-targets.json")
	if err != nil {
		t.Fatal(err)
	}
	sources, err := connectivitySources(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sources, "https://www.google.com") || strings.ContainsAny(sources, "*;") {
		t.Fatalf("unexpected sources %q", sources)
	}
	if sources != browserConnectivitySources {
		t.Fatal("policy did not load the shipped catalog")
	}
}
