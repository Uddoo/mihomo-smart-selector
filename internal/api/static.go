package api

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var fingerprintedAsset = regexp.MustCompile(`^assets/.+-[A-Za-z0-9_-]{8}\.(js|css|woff2)$`)

type staticRepresentation struct {
	data []byte
	etag string
}

type staticAsset struct {
	identity staticRepresentation
	gzip     staticRepresentation
}

func representation(data []byte) staticRepresentation {
	return staticRepresentation{data: data, etag: fmt.Sprintf(`"%x"`, sha256.Sum256(data))}
}

// Prepare metadata once; requests never compress or hash assets on the router.
func newStaticHandler(files fs.FS) (http.Handler, error) {
	assets := make(map[string]staticAsset)
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasSuffix(name, ".gz") {
			return nil
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		asset := staticAsset{identity: representation(data)}
		compressed, err := fs.ReadFile(files, name+".gz")
		if err == nil {
			asset.gzip = representation(compressed)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		assets[name] = asset
		return nil
	})
	if err != nil {
		return nil, err
	}
	fallback := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		w.Header().Set("Cache-Control", "no-cache")
		asset, ok := assets[name]
		if !ok || r.URL.Path == "/index.html" {
			fallback.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if fingerprintedAsset.MatchString(name) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		selected := asset.identity
		if asset.gzip.data != nil {
			w.Header().Add("Vary", "Accept-Encoding")
			if acceptsGzip(r.Header.Get("Accept-Encoding")) {
				selected = asset.gzip
				w.Header().Set("Content-Encoding", "gzip")
			}
		}
		w.Header().Set("ETag", selected.etag)
		if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(selected.data)))
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(selected.data))
	}), nil
}

func acceptsGzip(header string) bool {
	wildcard := false
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(value, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding != "gzip" && coding != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(key, "q") {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil || !(parsed >= 0 && parsed <= 1) {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		if coding == "gzip" {
			return quality > 0
		}
		wildcard = quality > 0
	}
	return wildcard
}
