package monitor

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestSevenDayWindowAndHourObservation(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 20, 0, time.UTC)
	now := start.Add(7 * 24 * time.Hour)
	random := rand.New(rand.NewSource(1))
	samples := []model.MonitorSample{}
	for i := 0; i < 7*24*30; i++ {
		if i%53 == 0 {
			continue
		}
		outcome := "success"
		if random.Intn(10) == 0 {
			outcome = "failure"
		}
		if i%73 == 0 {
			outcome = "unknown"
		}
		samples = append(samples, model.MonitorSample{Kind: "baseline", Slot: int64(i), At: start.Add(time.Duration(i*120) * time.Second), ScheduledAt: start.Unix() + int64(i*120), Outcome: outcome, DelayMS: 100 + random.Intn(1200)})
	}
	week := windowMetrics(samples, start.Unix(), now, 7*24*time.Hour)
	day := metrics(samples, start.Unix(), now)
	hour := windowMetrics(samples, start.Unix(), now, time.Hour)
	if week.Readiness != "ready" || week.Score == nil || week.Samples <= day.Samples || hour.Score != nil || hour.Readiness != "observational" {
		t.Fatal(week, day, hour)
	}
	points := trends(samples[len(samples)-20:], start.Unix(), now.Add(-time.Hour), now)
	if len(points) < 30 {
		t.Fatal("1h trend not detailed", len(points))
	}
	var count int
	for _, p := range points {
		count += p.Success + p.Failure + p.Unknown
		if p.Success == 0 && (p.P50 != nil || p.P95 != nil) {
			t.Fatal("missing latency became zero")
		}
	}
	if count < 30 {
		t.Fatal("gaps not represented", count)
	}
}

func TestBrowserWindowCannotChangeFailoverWindow(t *testing.T) {
	m, _, now := failoverFixture(t)
	week, err := m.WindowOverview(context.Background(), "7d")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := m.overview(context.Background(), "24h", false)
	if err != nil {
		t.Fatal(err)
	}
	if week.Window != "7d" || decision.Window != "24h" || len(decision.Events) != 0 {
		t.Fatal("decision view shares UI range")
	}
	if len(failoverCandidates(decision)) == 0 {
		t.Fatal("week view disabled decision")
	}
	if _, err = m.WindowOverview(context.Background(), "90d"); err == nil {
		t.Fatal("unbounded range accepted")
	}
	_ = now
}
