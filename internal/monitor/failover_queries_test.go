package monitor

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"modernc.org/sqlite"
)

// Only the two views in this test's private databases call this function.
var baselineSelects atomic.Int64
var registerBaselineCounter = sync.OnceValue(func() error {
	return sqlite.RegisterScalarFunction("test_count_baseline_read", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		baselineSelects.Add(1)
		return int64(1), nil
	})
})

func TestFailoverWithoutFreshAlternativeSkipsBaselineQueries(t *testing.T) {
	if err := registerBaselineCounter(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		size int
	}{{"single", 1}, {"unknown", 30}, {"stale", 30}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "monitor.db")
			m, _, f, now := setupAt(t, path)
			source := &fakeSwitchSource{fakeSource: f}
			m.source = source
			names := []string{f.current}
			for i := 1; i < tc.size; i++ {
				name := fmt.Sprintf("backup-%d", i)
				f.nodes = append(f.nodes, model.MonitorNode{ID: name, Name: name})
				names = append(names, name)
			}
			ctx := context.Background()
			on := true
			plan, err := m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Group: "ChatGPT", ProfileID: f.profile.ID, Enabled: true, AutoSwitch: &on, Nodes: names, CandidateLimit: &tc.size})
			if err != nil {
				t.Fatal(err)
			}
			// Materialize one counter call per SELECT, even for an empty series.
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, table := range []string{"monitor_observations", "monitor_hourly"} {
				_, err = db.Exec(`ALTER TABLE ` + table + ` RENAME TO original_` + table + `;
					CREATE VIEW ` + table + ` AS WITH reads AS MATERIALIZED (SELECT test_count_baseline_read() AS counted)
					SELECT data.* FROM reads CROSS JOIN original_` + table + ` AS data WHERE reads.counted`)
				if err != nil {
					t.Fatal(err)
				}
			}
			baselineSelects.Store(0)
			if _, err := m.overview(ctx, "24h", false); err != nil || baselineSelects.Load() != int64(2*tc.size) {
				t.Fatalf("uncached control read %d statements for %d nodes: %v", baselineSelects.Load(), tc.size, err)
			}
			baselineSelects.Store(0)
			for cycle := 0; cycle < 2; cycle++ {
				*now = now.Add(31 * time.Second)
				m.refresh(ctx, *plan, *now)
				m.states[plan.Nodes[0].ID] = model.MonitorState{Status: "unavailable", Failures: 2, LastAt: *now}
				for i, n := range plan.Nodes[1:] {
					if tc.name == "stale" {
						state := model.MonitorState{Status: "healthy", LastAt: *now, LastSuccess: *now}
						if i%2 == 0 {
							state.LastAt = now.Add(-241 * time.Second)
						} else {
							state.LastSuccess = now.Add(-241 * time.Second)
						}
						m.states[n.ID] = state
					}
				}
				m.failover(ctx, *now)
			}
			if got := baselineSelects.Load(); got != 0 || len(source.switches) != 0 || source.calls != 0 {
				t.Fatalf("two cooldowns: baseline SELECTs=%d probes=%d switches=%d", got, source.calls, len(source.switches))
			}
		})
	}
}
