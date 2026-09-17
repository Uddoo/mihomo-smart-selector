package history

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Opt-in regression against an existing database, with writes disabled at both
// the file-open and connection levels. It never creates fixtures or runs probes.
func TestMonitorEventsReadOnlyRegression(t *testing.T) {
	path := os.Getenv("MIHOMO_READONLY_DB")
	if path == "" {
		t.Skip("set MIHOMO_READONLY_DB to an existing database for read-only query verification")
	}
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(1000)"
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var readonly int
	if err = tx.QueryRowContext(ctx, "PRAGMA query_only").Scan(&readonly); err != nil || readonly != 1 {
		t.Fatalf("query-only mode not enabled: %v", err)
	}
	rs, err := tx.QueryContext(ctx, "SELECT DISTINCT plan_id FROM monitor_events")
	if err != nil {
		t.Fatal(err)
	}
	plans := []string{"__nonexistent_readonly_regression_plan__"}
	for rs.Next() {
		var plan string
		if err = rs.Scan(&plan); err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	if err = rs.Err(); err != nil {
		t.Fatal(err)
	}
	rs.Close()
	type event struct {
		ID      int64
		Payload string
	}
	read := func(query, plan string) []event {
		t.Helper()
		start := time.Now()
		r, e := tx.QueryContext(ctx, query, plan)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Close()
		out := []event{}
		for r.Next() {
			var item event
			if e = r.Scan(&item.ID, &item.Payload); e != nil {
				t.Fatal(e)
			}
			out = append(out, item)
		}
		if e = r.Err(); e != nil {
			t.Fatal(e)
		}
		t.Logf("query read %d rows in %s", len(out), time.Since(start))
		return out
	}
	for _, plan := range plans {
		old := read(`SELECT id,payload FROM monitor_events WHERE plan_id=? ORDER BY id DESC LIMIT 100`, plan)
		got := read(monitorEventsQuery, plan)
		if !reflect.DeepEqual(got, old) {
			t.Fatal("latest event identities, payloads or ordering changed")
		}
		if len(got) > 100 {
			t.Fatal("event query is no longer bounded")
		}
		for i := 1; i < len(got); i++ {
			if got[i-1].ID <= got[i].ID {
				t.Fatal("event order changed")
			}
		}
	}
	// On both the original schema and the migrated schema, JSON is loaded only
	// through rowid lookups after the candidate identities have been limited.
	rs, err = tx.QueryContext(ctx, "EXPLAIN QUERY PLAN "+monitorEventsQuery, plans[len(plans)-1])
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var details []string
	for rs.Next() {
		var a, b, c int
		var detail string
		if err = rs.Scan(&a, &b, &c, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err = rs.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "\n")
	if !strings.Contains(plan, "INTEGER PRIMARY KEY") || !strings.Contains(plan, "COVERING INDEX") {
		t.Fatalf("event payloads are not fetched through bounded identities:\n%s", plan)
	}
	t.Log(plan)
}
