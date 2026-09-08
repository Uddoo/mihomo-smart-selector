package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestMonitorAtomicDeduplicationAndRetention(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	p := model.MonitorPlan{ID: "plan", Revision: 1, CreatedAt: now}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	sample := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 0, At: now, Outcome: "failure"}
	state := model.MonitorState{Status: "suspect", Failures: 1, LastAt: now}
	event := &model.MonitorEvent{NodeID: "a", At: now, Status: "suspect"}
	if added, err := s.RecordMonitor(ctx, p.ID, sample, state, event); err != nil || !added {
		t.Fatal(added, err)
	}
	sample.Outcome = "success"
	state.Status = "healthy"
	event.Status = "healthy"
	if added, err := s.RecordMonitor(ctx, p.ID, sample, state, event); err != nil || added {
		t.Fatal("duplicate overwrote sample", added, err)
	}
	states, err := s.MonitorStates(ctx, p.ID)
	if err != nil || states["a"].Status != "suspect" {
		t.Fatal(states, err)
	}
	events, err := s.MonitorEvents(ctx, p.ID)
	if err != nil || len(events) != 1 || events[0].Status != "suspect" {
		t.Fatal(events, err)
	}
	sample.Slot = 1
	sample.At = now.Add(-8 * 24 * time.Hour)
	event.At = sample.At
	if _, err = s.RecordMonitor(ctx, p.ID, sample, state, event); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	samples, err := s.MonitorSamples(ctx, p.ID, now.Add(-30*24*time.Hour))
	if err != nil || len(samples) != 1 || samples[0].Outcome != "failure" {
		t.Fatal(samples, err)
	}
}
