package scan

import (
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestIndexedResultsPreserveSamplesAfterSnapshotReplacement(t *testing.T) {
	m := NewManager(config.Defaults(), nil, nil)
	s := model.Scan{ID: "scan", StartedAt: time.Now(), Results: []model.NodeResult{{Name: "A", ScreeningSamples: 1, Samples: []model.ProbeSample{{DelayMS: 100}}}, {Name: "B"}}}
	m.setActive(s)
	profile := config.ProbeProfile{ID: "test"}
	r, _ := m.recordCandidate(s.ID, model.NodeResult{Name: "A", Samples: []model.ProbeSample{{DelayMS: 200}}}, profile)
	if len(r.Samples) != 2 || r.Samples[0].DelayMS != 100 || r.ScreeningSamples != 1 {
		t.Fatal("merge lost screening evidence", r)
	}
	s.Results = []model.NodeResult{{Name: "B"}, {Name: "A", Samples: []model.ProbeSample{{DelayMS: 300}}}}
	m.setActive(s)
	r, _ = m.recordCandidate(s.ID, model.NodeResult{Name: "A", Samples: []model.ProbeSample{{DelayMS: 400}}}, profile)
	if len(r.Samples) != 2 || r.Samples[0].DelayMS != 300 || m.active[s.ID].Results[0].Name != "B" {
		t.Fatal("stale result index", r)
	}
	m.recordCandidate(s.ID, model.NodeResult{Name: "C"}, profile)
	if len(m.active[s.ID].Results) != 3 {
		t.Fatal("new result missing")
	}
	m.removeActive(s.ID)
	if _, ok := m.resultIndex[s.ID]; ok {
		t.Fatal("completed scan retained index")
	}
}
