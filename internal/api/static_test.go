package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func staticFixture(t *testing.T) (http.Handler, []byte) {
	t.Helper()
	raw := []byte(strings.Repeat("export const app = 'fixture';\n", 100))
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	if _, err := zip.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	handler, err := newStaticHandler(fstest.MapFS{
		"index.html":                 {Data: []byte("<!doctype html><h1>fixture</h1>")},
		"assets/app-12345678.js":     {Data: raw},
		"assets/app-12345678.js.gz":  {Data: compressed.Bytes()},
		"assets/font-12345678.woff2": {Data: []byte("fixture font")},
		"service-icons/example.svg":  {Data: []byte("<svg/>")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler, raw
}

func staticRequest(handler http.Handler, method, url string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestStaticCompressionAndConditionalRequests(t *testing.T) {
	handler, raw := staticFixture(t)
	url := "/assets/app-12345678.js"
	identity := staticRequest(handler, "GET", url, nil)
	compressed := staticRequest(handler, "GET", url, map[string]string{"Accept-Encoding": "gzip, br"})
	if compressed.Code != 200 || compressed.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(compressed.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("gzip response: %d %v", compressed.Code, compressed.Header())
	}
	if compressed.Body.Len() >= len(raw) {
		t.Fatal("gzip did not reduce payload")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(decoded, raw) || !bytes.Equal(identity.Body.Bytes(), raw) {
		t.Fatalf("representation mismatch: %v", err)
	}
	if identity.Header().Get("Vary") != "Accept-Encoding" || compressed.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatal("encoding must vary in both representations")
	}
	if identity.Header().Get("ETag") == compressed.Header().Get("ETag") {
		t.Fatal("representations need distinct strong validators")
	}
	for _, encoding := range []string{"identity", "gzip"} {
		initial := staticRequest(handler, "GET", url, map[string]string{"Accept-Encoding": encoding})
		if !strings.Contains(initial.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("hashed resource is not immutable")
		}
		head := staticRequest(handler, "HEAD", url, map[string]string{"Accept-Encoding": encoding})
		if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != initial.Header().Get("Content-Length") {
			t.Fatalf("HEAD mismatch for %s", encoding)
		}
		notModified := staticRequest(handler, "GET", url, map[string]string{"Accept-Encoding": encoding, "If-None-Match": initial.Header().Get("ETag")})
		if notModified.Code != 304 || notModified.Body.Len() != 0 {
			t.Fatalf("conditional %s = %d", encoding, notModified.Code)
		}
	}
	mismatch := staticRequest(handler, "GET", url, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": identity.Header().Get("ETag")})
	if mismatch.Code != 200 {
		t.Fatal("identity validator incorrectly matched gzip")
	}
	partial := staticRequest(handler, "GET", url, map[string]string{"Range": "bytes=0-9"})
	if partial.Code != 206 || !bytes.Equal(partial.Body.Bytes(), raw[:10]) {
		t.Fatal("byte ranges regressed")
	}
}

func TestStaticCacheScopeAndNegotiation(t *testing.T) {
	handler, _ := staticFixture(t)
	for _, url := range []string{"/", "/service-icons/example.svg", "/missing.js", "/assets/missing-12345678.js"} {
		response := staticRequest(handler, "GET", url, nil)
		if strings.Contains(response.Header().Get("Cache-Control"), "immutable") || (response.Code == 200 && response.Header().Get("Cache-Control") != "no-cache") {
			t.Fatalf("%s was given immutable caching", url)
		}
	}
	font := staticRequest(handler, "GET", "/assets/font-12345678.woff2", map[string]string{"Accept-Encoding": "gzip"})
	if font.Code != 200 || font.Header().Get("Content-Encoding") != "" || !strings.Contains(font.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("uncompressed font fallback failed")
	}
	for header, want := range map[string]bool{"": false, "br": false, "gzip": true, "gzip;q=0": false, "gzip;q=0.5": true, "*;q=1,gzip;q=0": false, "*;q=0.5": true, "gzip;q=oops": false, "gzip;q=2": false, "gzip;q=NaN": false, "GZip;Q=0.8": true} {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v", header, got)
		}
	}
	response := staticRequest(handler, "POST", "/assets/app-12345678.js", nil)
	if response.Code != 405 {
		t.Fatal("mutation method served asset")
	}
}

func TestEmbeddedPrecompressedAssetsMatch(t *testing.T) {
	files, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	err = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".gz") {
			return err
		}
		compressed, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return err
		}
		decoded, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return err
		}
		raw, err := fs.ReadFile(files, strings.TrimSuffix(name, ".gz"))
		if err != nil {
			return err
		}
		if !bytes.Equal(decoded, raw) {
			t.Errorf("stale compressed asset: %s", name)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("production build has no precompressed JS/CSS")
	}
}
