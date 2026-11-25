package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestScanReadErrors(t *testing.T) {
	f := newScanMonitorFixture(t)
	source := f.sourceScan()
	f.insertScan(t, source)
	path := "/api/v1/scans/" + source.ID
	checkScan := func(t *testing.T) {
		t.Helper()
		w := f.call(t, http.MethodGet, path, nil)
		var scan model.Scan
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &scan) != nil || scan.ID != source.ID {
			t.Fatalf("read stored scan: %d %s", w.Code, w.Body.String())
		}
	}
	checkErrors := func(t *testing.T, path string, status int, message string) {
		t.Helper()
		for _, suffix := range []string{"", "/events"} {
			w := f.call(t, http.MethodGet, path+suffix, nil)
			var body map[string]string
			if w.Code != status || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body) != 1 || body["error"] != message {
				t.Errorf("GET %s: %d %s, want %d %q", path+suffix, w.Code, w.Body.String(), status, message)
			}
		}
	}
	checkScan(t)
	t.Run("missing record", func(t *testing.T) {
		checkErrors(t, "/api/v1/scans/missing", http.StatusNotFound, "scan not found")
	})
	t.Run("unreadable record then recovery", func(t *testing.T) {
		db, err := sql.Open("sqlite", f.dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`UPDATE scans SET warnings_json = 'invalid-json' WHERE id = ?`, source.ID); err != nil {
			t.Fatal(err)
		}
		checkErrors(t, path, http.StatusInternalServerError, "could not read scan")
		if _, err := db.Exec(`UPDATE scans SET warnings_json = '[]' WHERE id = ?`, source.ID); err != nil {
			t.Fatal(err)
		}
		checkScan(t)
	})
	t.Run("closed database", func(t *testing.T) {
		if err := f.store.Close(); err != nil {
			t.Fatal(err)
		}
		checkErrors(t, path, http.StatusInternalServerError, "could not read scan")
	})
}
