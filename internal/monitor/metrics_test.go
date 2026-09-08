package monitor

import (
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestLongTermScoreKeepsFailuresAndIgnoresRetests(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	stable, flaky := []model.MonitorSample{}, []model.MonitorSample{}
	for i := 0; i < 720; i++ {
		s := model.MonitorSample{Kind: "baseline", Slot: int64(i), At: start.Add(time.Duration(i*Interval) * time.Second), Outcome: "success", DelayMS: 400}
		stable = append(stable, s)
		s.DelayMS = 80
		if i%30 == 0 {
			s.Outcome = "failure"
		}
		flaky = append(flaky, s)
	}
	a := metrics(stable, start.Unix(), start.Add(24*time.Hour-time.Second))
	b := metrics(flaky, start.Unix(), start.Add(24*time.Hour-time.Second))
	if a.Score == nil || b.Score == nil || *a.Score <= *b.Score {
		t.Fatalf("stable must outrank fast/flaky: %+v %+v", a, b)
	}
	before := *b.Score
	for i := 0; i < 2000; i++ {
		flaky = append(flaky, model.MonitorSample{Kind: "manual", Outcome: "success", DelayMS: 1, At: start.Add(23 * time.Hour), Slot: int64(i)})
	}
	after := metrics(flaky, start.Unix(), start.Add(24*time.Hour-time.Second))
	if *after.Score != before || after.Incidents != 24 || after.Samples != 720 {
		t.Fatalf("retests rewrote evidence: %+v", after)
	}
}

func TestMetricsMissingSlotsWindowAndReadiness(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	now := start.Add(24 * time.Hour)
	samples := []model.MonitorSample{}
	for i := 0; i <= 720; i++ {
		outcome := "success"
		if i%2 == 0 {
			outcome = "unknown"
		}
		samples = append(samples, model.MonitorSample{Kind: "baseline", Slot: int64(i), At: start.Add(time.Duration(i*Interval) * time.Second), Outcome: outcome, DelayMS: 200})
	}
	s := metrics(samples, start.Unix(), now)
	if s.Readiness != "provisional" || s.Coverage >= .8 || s.SuccessRate != 1 {
		t.Fatalf("missing slots fabricated failures or full coverage: %+v", s)
	}
	for i := range samples {
		samples[i].Outcome = "success"
	}
	s = metrics(samples, start.Unix(), now)
	if s.Readiness != "ready" || s.Coverage != 1 {
		t.Fatalf("full window %+v", s)
	}
	// Duplicate slot cannot change its first failure into a retry success.
	samples[1].Outcome = "failure"
	samples = append(samples, model.MonitorSample{Kind: "baseline", Slot: 1, Outcome: "success", DelayMS: 1})
	s = metrics(samples, start.Unix(), now)
	if s.Samples != 721 || s.Incidents != 1 {
		t.Fatalf("duplicate counted: %+v", s)
	}
	s = metrics(samples, start.Unix(), now.Add(25*time.Hour))
	if s.Samples != 0 || s.Score != nil || s.Readiness != "collecting" {
		t.Fatalf("old window leaked: %+v", s)
	}
}

func TestRecoveryRequiresSeparatedSuccessAndGapIsUnknown(t *testing.T) {
	now := time.Unix(1700000040, 0).UTC()
	s := model.MonitorState{}
	apply := func(offset int, outcome string) {
		s = transition(s, model.MonitorSample{At: now.Add(time.Duration(offset) * time.Second), Outcome: outcome})
	}
	apply(0, "failure")
	if s.Status != "suspect" {
		t.Fatal(s)
	}
	apply(10, "failure")
	if s.Status != "unavailable" {
		t.Fatal(s)
	}
	apply(20, "success")
	apply(30, "success")
	apply(40, "success")
	if s.Status != "recovering" {
		t.Fatal("recovered from burst", s)
	}
	apply(140, "success")
	if s.Status != "healthy" {
		t.Fatal(s)
	}
	apply(150, "unknown")
	if s.Status != "unknown" || s.Failures != 0 {
		t.Fatal(s)
	}
	apply(160, "failure")
	apply(500, "failure")
	if s.Status != "suspect" {
		t.Fatal("gap counted consecutive failures", s)
	}
}
