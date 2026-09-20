package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestTaskConfigurationDoesNotInterruptRuntimeOwner(t *testing.T) {
	m, store, f, now := setup(t)
	ctx := context.Background()
	m.step(ctx, *now)
	original := clonePlan(m.plan)
	before := m.states[original.Nodes[0].ID]
	childCancelled := false
	m.inflight = func() { childCancelled = true }
	second, err := m.SaveTask(ctx, "", model.MonitorRequest{Group: "Video", ProfileID: "chatgpt", Nodes: []string{"A"}})
	if err != nil {
		t.Fatal(err)
	}
	if second.TaskID == "" || second.TaskID == original.TaskID || second.Enabled {
		t.Fatal(second)
	}
	if childCancelled || !reflect.DeepEqual(original, m.plan) || m.states[original.Nodes[0].ID] != before {
		t.Fatal("new task changed running state")
	}
	if second.Nodes[0].SeriesID == original.Nodes[0].SeriesID {
		t.Fatal("shared node leaked history")
	}
	if _, err = m.Save(ctx, model.MonitorRequest{Revision: original.Revision, Enabled: false}); !errors.Is(err, history.ErrMonitorTaskRequired) {
		t.Fatal("ambiguous legacy save", err)
	}
	if childCancelled {
		t.Fatal("rejected operation cancelled active probe")
	}
	limit := 10
	on := true
	updated, err := m.SaveTask(ctx, second.TaskID, model.MonitorRequest{Revision: second.Revision, Group: "Video", ProfileID: "chatgpt", Nodes: []string{"A"}, CandidateLimit: &limit, AutoSwitch: &on})
	if err != nil || updated.TaskID != second.TaskID || !updated.AutoSwitch || updated.CandidateLimit != 10 {
		t.Fatal(updated, err)
	}
	if childCancelled {
		t.Fatal("inactive edit cancelled active probe")
	}
	f.nodes = append(f.nodes, model.MonitorNode{ID: "b", Name: "B", Provider: "P", Protocol: "VLESS"})
	changed, err := m.SaveTask(ctx, second.TaskID, model.MonitorRequest{Revision: updated.Revision, Group: "Video", ProfileID: "chatgpt", Nodes: []string{"A", "B"}})
	if err != nil || changed.TaskID != second.TaskID || changed.ID == second.ID || changed.Nodes[0].SeriesID != second.Nodes[0].SeriesID {
		t.Fatal("task/evidence identity", changed, err)
	}
	if !reflect.DeepEqual(original, m.plan) {
		t.Fatal("inactive edit changed active plan")
	}
	*now = now.Add(120 * time.Second)
	m.inflight = nil
	m.step(ctx, *now)
	if f.calls != 2 {
		t.Fatal("original task stopped or another task ran", f.calls)
	}
	reloaded, err := New(store, f)
	if err != nil || reloaded.plan.TaskID != original.TaskID || !reloaded.plan.Enabled {
		t.Fatal("runtime owner not restored", err)
	}
	paused, err := m.SaveTask(ctx, original.TaskID, model.MonitorRequest{Revision: original.Revision, Enabled: false})
	if err != nil || paused.Enabled {
		t.Fatal("explicit pause", paused, err)
	}
	other, err := m.Task(ctx, second.TaskID)
	if err != nil || !other.Scheduled || other.Plan.Revision != changed.Revision || !other.Plan.AutoSwitch {
		t.Fatal("other task changed", other, err)
	}
}

func TestTaskEditsRequireIdentityAndFreshRevision(t *testing.T) {
	m, _, f, _ := setup(t)
	ctx := context.Background()
	p := clonePlan(m.plan)
	if _, err := m.SaveTask(ctx, "missing", model.MonitorRequest{Revision: p.Revision}); !errors.Is(err, history.ErrMonitorTaskNotFound) {
		t.Fatal(err)
	}
	if _, err := m.SaveTask(ctx, "", model.MonitorRequest{Group: p.Group, ProfileID: p.ProfileID, Nodes: []string{"A"}}); !errors.Is(err, history.ErrMonitorGroupExists) {
		t.Fatal(err)
	}
	f.nodes = append(f.nodes, model.MonitorNode{ID: "b", Name: "B"})
	changed, err := m.SaveTask(ctx, p.TaskID, model.MonitorRequest{Revision: p.Revision, Group: p.Group, ProfileID: p.ProfileID, Nodes: []string{"A", "B"}, Enabled: true})
	if err != nil || changed.TaskID != p.TaskID || changed.ID == p.ID {
		t.Fatal(changed, err)
	}
	if _, err = m.SaveTask(ctx, p.TaskID, model.MonitorRequest{Revision: p.Revision, Enabled: false}); err == nil {
		t.Fatal("stale task revision accepted")
	}
	// Pausing the scheduled task must keep working when discovery is offline.
	f.err = errors.New("offline")
	if _, err = m.SaveTask(ctx, p.TaskID, model.MonitorRequest{Revision: changed.Revision, Enabled: false}); err != nil {
		t.Fatal(err)
	}
}
