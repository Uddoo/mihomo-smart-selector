package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Build the actual pre-task table layout, retaining any seeded P2 evidence.
func useLegacyMonitorTables(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.db.Exec(`
ALTER TABLE monitor_plans RENAME TO fixture_plans;
ALTER TABLE monitor_revisions RENAME TO fixture_revisions;
CREATE TABLE monitor_plans (scope TEXT PRIMARY KEY,revision INTEGER NOT NULL,payload TEXT NOT NULL);
CREATE TABLE monitor_revisions (scope TEXT NOT NULL,revision INTEGER NOT NULL,plan_id TEXT NOT NULL,at INTEGER NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(scope,revision));
INSERT INTO monitor_plans SELECT scope,revision,json_remove(payload,'$.task_id') FROM fixture_plans;
INSERT INTO monitor_revisions SELECT scope,revision,plan_id,at,json_remove(payload,'$.task_id') FROM fixture_revisions;
DROP TABLE fixture_plans;
DROP TABLE fixture_revisions;
`)
	if err != nil {
		t.Fatal(err)
	}
}

func taskPlan(id, group string, at time.Time) model.MonitorPlan {
	return model.MonitorPlan{TaskID: id, ID: id + "-epoch", Revision: 1, Group: group, CandidateLimit: 6, ProfileID: "chatgpt", ProfileHash: "hash", CreatedAt: at, Nodes: []model.MonitorNode{{ID: "a", Name: "A", Provider: "P", Protocol: "VLESS"}}}
}

func TestMonitorTasksIsolationRevisionAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tasks.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	start := time.Now().UTC().Truncate(time.Second)
	a, b := taskPlan("a", "ChatGPT", start), taskPlan("b", "Video", start)
	a.Enabled, a.AutoSwitch = true, true
	for _, p := range []*model.MonitorPlan{&a, &b} {
		if err = s.PrepareMonitorPlan(ctx, "scope", p); err != nil {
			t.Fatal(err)
		}
		if err = s.SaveMonitorTask(ctx, "scope", *p, 0); err != nil {
			t.Fatal(err)
		}
	}
	if a.Nodes[0].SeriesID == b.Nodes[0].SeriesID {
		t.Fatal("tasks share evidence")
	}
	if _, err = s.MonitorPlan(ctx, "scope"); !errors.Is(err, ErrMonitorTaskRequired) {
		t.Fatal(err)
	}
	a.Revision++
	if err = s.SaveMonitorPlan(ctx, "scope", a, 1); !errors.Is(err, ErrMonitorTaskRequired) {
		t.Fatal("implicit save accepted", err)
	}
	if err = s.SaveMonitorTask(ctx, "scope", a, 1); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveMonitorTask(ctx, "scope", a, 1); !errors.Is(err, ErrMonitorRevision) {
		t.Fatal("stale edit", err)
	}
	duplicate := taskPlan("c", "Video", start)
	if err = s.SaveMonitorTask(ctx, "scope", duplicate, 0); !errors.Is(err, ErrMonitorGroupExists) {
		t.Fatal("duplicate group", err)
	}
	if _, err = s.MonitorTask(ctx, "other", a.TaskID); !errors.Is(err, ErrMonitorTaskNotFound) {
		t.Fatal("cross-controller read", err)
	}
	if err = s.SaveMonitorTask(ctx, "other", duplicate, 0); err != nil {
		t.Fatal("group should be scoped", err)
	}
	for i, p := range []model.MonitorPlan{a, b} {
		sample := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 0, At: start, Outcome: "success", DelayMS: (i + 1) * 100}
		if added, e := s.RecordMonitor(ctx, p.ID, sample, model.MonitorState{Status: "healthy", LastAt: start}, nil); e != nil || !added {
			t.Fatal(added, e)
		}
	}
	before, err := s.MonitorTasks(ctx, "scope")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.MonitorTasks(ctx, "scope")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("restart changed tasks", before, after, err)
	}
	owner, err := s.RuntimeMonitorPlan(ctx, "scope")
	if err != nil || owner.TaskID != a.TaskID || !owner.Enabled || !owner.AutoSwitch {
		t.Fatal(owner, err)
	}
	for i, p := range []model.MonitorPlan{a, b} {
		r, e := s.MonitorTaskRevisions(ctx, "scope", p.TaskID)
		if e != nil || len(r) != 2-i || r[0].Plan.TaskID != p.TaskID {
			t.Fatal(r, e)
		}
		items, e := s.MonitorSamples(ctx, p.ID, start)
		if e != nil || len(items) != 1 || items[0].DelayMS != (i+1)*100 {
			t.Fatal("evidence crossed tasks", items, e)
		}
	}
}

func TestMonitorTaskConcurrentRevisionOnlyOneWriter(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := taskPlan("a", "g", time.Now().UTC())
	if err = s.SaveMonitorTask(context.Background(), "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	p.Revision = 2
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.SaveMonitorTask(context.Background(), "scope", p, 1) }()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrMonitorRevision) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal(success, stale)
	}
}

func TestMonitorTaskMigrationPreservesP2EvidenceAndRollsBack(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	useLegacyMonitorTables(t, s)
	start := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	p := taskPlan("", "legacy-group", start)
	p.ID, p.TaskID, p.Revision, p.AutoSwitch = "legacy-epoch", "", 7, true
	p.Enabled = false
	data, _ := json.Marshal(p)
	if _, err = s.db.Exec(`INSERT INTO monitor_plans VALUES(?,?,?)`, "scope", 7, string(data)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`INSERT INTO monitor_revisions VALUES(?,?,?,?,?)`, "scope", 7, p.ID, start.Unix(), string(data)); err != nil {
		t.Fatal(err)
	}
	earlier := p
	earlier.ID, earlier.Revision, earlier.Group = "earlier-epoch", 6, "earlier-group"
	earlier.CreatedAt = start.Add(-time.Hour)
	earlierData, _ := json.Marshal(earlier)
	if _, err = s.db.Exec(`INSERT INTO monitor_revisions VALUES(?,?,?,?,?)`, "scope", 6, earlier.ID, earlier.CreatedAt.Unix(), string(earlierData)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	preserved, err := s.MonitorPlan(ctx, "scope")
	if err != nil {
		t.Fatal(err)
	}
	v := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 0, At: start, Outcome: "failure"}
	state := model.MonitorState{Status: "unavailable", LastAt: start, Failures: 2}
	event := &model.MonitorEvent{NodeID: "a", NodeName: "A", At: start, Status: "unavailable"}
	if added, e := s.RecordMonitor(ctx, p.ID, v, state, event); e != nil || !added {
		t.Fatal(added, e)
	}
	if _, err = s.AggregateMonitorBatch(ctx, time.Now().UTC(), 4); err != nil {
		t.Fatal(err)
	}
	var observations, hourly, events int
	for table, dest := range map[string]*int{"monitor_observations": &observations, "monitor_hourly": &hourly, "monitor_events": &events} {
		if err = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if observations != 1 || hourly != 1 || events != 1 {
		t.Fatal(observations, hourly, events)
	}
	useLegacyMonitorTables(t, s)
	// A corrupt immutable revision must roll back both rebuilt tables.
	if _, err = s.db.Exec(`UPDATE monitor_revisions SET payload='invalid' WHERE revision=7`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if broken, e := Open(path); e == nil {
		broken.Close()
		t.Fatal("corrupt migration accepted")
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var columns int
	if err = raw.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('monitor_plans') WHERE name='task_id'`).Scan(&columns); err != nil || columns != 0 {
		t.Fatal("migration was not atomic", columns, err)
	}
	if _, err = raw.Exec(`UPDATE monitor_revisions SET payload=? WHERE revision=7`, string(data)); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	for i := 0; i < 2; i++ {
		s, err = Open(path)
		if err != nil {
			t.Fatal(err)
		}
		got, e := s.MonitorPlan(ctx, "scope")
		if e != nil || !reflect.DeepEqual(got, preserved) {
			t.Fatal("migration changed plan/series/anchor", got, preserved, e)
		}
		revs, e := s.MonitorTaskRevisions(ctx, "scope", got.TaskID)
		if e != nil || len(revs) != 2 || revs[0].Plan.ID != p.ID || revs[0].Plan.Revision != 7 || revs[1].Plan.ID != earlier.ID || revs[1].Plan.TaskID != got.TaskID || revs[1].At.Unix() != earlier.CreatedAt.Unix() {
			t.Fatal(revs, e)
		}
		checkpoint, _, e := s.MonitorCheckpoint(ctx, got.Nodes)
		if e != nil || checkpoint["a"].Failures != 2 {
			t.Fatal(checkpoint, e)
		}
		items, e := s.MonitorSamples(ctx, p.ID, start)
		if e != nil || len(items) != 1 || items[0].Outcome != "failure" {
			t.Fatal(items, e)
		}
		for table, want := range map[string]int{"monitor_observations": observations, "monitor_hourly": hourly, "monitor_events": events} {
			var count int
			if e = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); e != nil || count != want {
				t.Fatal(table, count, e)
			}
		}
		s.Close()
	}
}

func TestMonitorTaskRevisionEventsHaveDistinctPaginationKeys(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second)
	for _, id := range []string{"a", "b", "c"} {
		if err = s.SaveMonitorTask(ctx, "scope", taskPlan(id, id, at), 0); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 4; i++ {
		page, e := s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), cursor, 1)
		if e != nil {
			t.Fatal(e)
		}
		for _, event := range page.Items {
			if seen[event.Key] {
				t.Fatal("duplicate revision event", event)
			}
			seen[event.Key] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 3 {
		t.Fatal("per-task revision 1 collided", seen)
	}
}

func TestLegacyTaskRevisionPreservesAbsentAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-fields.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	useLegacyMonitorTables(t, s)
	p := taskPlan("", "g", time.Now().UTC())
	p.TaskID = ""
	current, _ := json.Marshal(p)
	if _, err = s.db.Exec(`INSERT INTO monitor_plans VALUES(?,?,?)`, "scope", 1, string(current)); err != nil {
		t.Fatal(err)
	}
	var old map[string]json.RawMessage
	if err = json.Unmarshal(current, &old); err != nil {
		t.Fatal(err)
	}
	delete(old, "candidate_limit")
	delete(old, "task_id")
	old["legacy_extension"] = json.RawMessage(`{"integer":9007199254740993,"label":"retained"}`)
	payload, _ := json.Marshal(old)
	if _, err = s.db.Exec(`INSERT INTO monitor_revisions VALUES(?,?,?,?,?)`, "scope", 1, p.ID, p.CreatedAt.Unix(), string(payload)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	for i := 0; i < 2; i++ {
		s, err = Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var data string
		if err = s.db.QueryRow(`SELECT payload FROM monitor_revisions WHERE scope=? AND revision=1`, "scope").Scan(&data); err != nil {
			t.Fatal(err)
		}
		var got map[string]json.RawMessage
		if err = json.Unmarshal([]byte(data), &got); err != nil {
			t.Fatal(err)
		}
		if len(got["task_id"]) == 0 {
			t.Fatal("task attribution missing")
		}
		delete(got, "task_id")
		if !reflect.DeepEqual(old, got) {
			t.Fatal("migration rewrote historical fields")
		}
		s.Close()
	}
}
