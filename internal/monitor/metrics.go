package monitor

import (
	"math"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

const Interval = 120 // seconds; v1 baseline cadence is immutable within a plan

func transition(prev model.MonitorState, sample model.MonitorSample) model.MonitorState {
	s := prev
	if s.LastAt.IsZero() || sample.At.Sub(s.LastAt) > 2*Interval*time.Second || sample.At.Before(s.LastAt) {
		s = model.MonitorState{Status: "unknown", LastSuccess: prev.LastSuccess}
	}
	s.LastAt = sample.At
	if sample.Outcome == "unknown" {
		s.Status = "unknown"
		s.Failures = 0
		s.Successes = 0
		return s
	}
	if sample.Outcome == "failure" {
		s.Failures++
		s.Successes = 0
		if s.Failures >= 2 || !s.IncidentStart.IsZero() {
			s.Status = "unavailable"
			if s.IncidentStart.IsZero() {
				s.IncidentStart = sample.At
			}
		} else {
			s.Status = "suspect"
		}
		return s
	}
	s.LastSuccess = sample.At
	s.Failures = 0
	s.Successes++
	if !s.IncidentStart.IsZero() || prev.Status == "unavailable" || prev.Status == "recovering" {
		if prev.Status != "recovering" {
			s.RecoverySlot = sample.At.Unix() / Interval
			s.Successes = 1
		}
		s.Status = "recovering"
		if s.Successes >= 3 && sample.At.Unix()/Interval > s.RecoverySlot {
			s.Status = "healthy"
			s.IncidentStart = time.Time{}
		}
	} else {
		s.Status = "healthy"
	}
	return s
}

func nodeAnchor(p model.MonitorPlan, index int) int64 {
	if p.Nodes[index].Anchor != 0 {
		return p.Nodes[index].Anchor
	}
	return p.CreatedAt.Unix() + int64(index*Interval/len(p.Nodes))
}

// Expected slots include gaps (including browser absence, pauses and restarts).
// Extra checks are never allowed to replace a failed baseline with a success.
func metrics(samples []model.MonitorSample, anchor int64, now time.Time) model.MonitorMetrics {
	return windowMetrics(samples, anchor, now, 24*time.Hour)
}

func windowMetrics(samples []model.MonitorSample, anchor int64, now time.Time, window time.Duration) model.MonitorMetrics {
	// Generic callers may supply unordered/duplicate evidence. History queries
	// already guarantee unique, ordered baseline slots and need no normalization.
	ordered := true
	last := int64(-1)
	for _, s := range samples {
		if s.Kind != "baseline" || s.Slot <= last {
			ordered = false
			break
		}
		last = s.Slot
	}
	if !ordered {
		indices := make([]int, 0, len(samples))
		for i := range samples {
			if samples[i].Kind == "baseline" {
				indices = append(indices, i)
			}
		}
		sort.SliceStable(indices, func(i, j int) bool { return samples[indices[i]].Slot < samples[indices[j]].Slot })
		baseline := make([]model.MonitorSample, 0, len(indices))
		for _, i := range indices {
			if len(baseline) == 0 || baseline[len(baseline)-1].Slot != samples[i].Slot {
				baseline = append(baseline, samples[i])
			}
		}
		samples = baseline
	}
	return orderedWindowMetrics(samples, anchor, now, window)
}

func orderedWindowMetrics(samples []model.MonitorSample, anchor int64, now time.Time, window time.Duration) model.MonitorMetrics {
	accumulator := newWindowAccumulator(anchor, now, window, len(samples))
	for _, sample := range samples {
		accumulator.add(sample.Slot, sample.Outcome, sample.DelayMS)
	}
	return accumulator.finish()
}

type windowAccumulator struct {
	out                                    model.MonitorMetrics
	anchor, now, first, last, previousSlot int64
	delays                                 []int
	successes                              int
	failed                                 bool
}

func newWindowAccumulator(anchor int64, now time.Time, window time.Duration, capacity int) windowAccumulator {
	out := model.MonitorMetrics{Readiness: "collecting", WindowSeconds: int64(window / time.Second), ObservedSeconds: min(int64(window/time.Second), max(int64(0), now.Unix()-anchor))}
	start := max(anchor, now.Add(-window).Unix())
	first := max(int64(0), (start-anchor+Interval-1)/Interval)
	last := (now.Unix() - anchor) / Interval
	if now.Unix() >= anchor && last >= first {
		out.Expected = int(last - first + 1)
	}
	return windowAccumulator{out: out, anchor: anchor, now: now.Unix(), first: first, last: last, previousSlot: -2, delays: make([]int, 0, capacity)}
}

func (a *windowAccumulator) add(slot int64, outcome string, delay int) {
	if slot < a.first || slot > a.last {
		return
	}
	if outcome == "unknown" {
		a.failed = false
		a.previousSlot = slot
		return
	}
	a.out.Samples++
	if outcome == "success" {
		a.successes++
		a.delays = append(a.delays, delay)
		a.failed = false
	} else {
		a.out.FailureSeconds += int(min(int64(Interval), max(int64(0), a.now-(a.anchor+slot*Interval))))
		if !a.failed || slot != a.previousSlot+1 {
			a.out.Incidents++
		}
		a.failed = true
	}
	a.previousSlot = slot
}

func (a *windowAccumulator) finish() model.MonitorMetrics {
	out, delays, successes := a.out, a.delays, a.successes
	if out.Expected > 0 {
		out.Coverage = float64(out.Samples) / float64(out.Expected)
	}
	if out.Samples > 0 {
		out.SuccessRate = float64(successes) / float64(out.Samples)
	}
	if len(delays) > 0 {
		sort.Ints(delays)
		out.P95MS = delays[int(math.Ceil(.95*float64(len(delays))))-1]
	}
	if out.WindowSeconds <= int64(time.Hour/time.Second) {
		out.Readiness = "observational"
		return out
	}
	if out.Samples < 100 {
		return out
	}
	continuity := math.Max(0, 1-float64(out.Incidents)/(float64(out.Samples*Interval)/21600))
	latency := 0.0
	if out.P95MS > 0 {
		latency = math.Max(0, 1-float64(out.P95MS)/1000)
	}
	score := math.Round((70*out.SuccessRate+20*continuity+10*latency)*10) / 10
	if successes == 0 {
		score = 0
	}
	out.AvailabilityPoints = math.Round(70*out.SuccessRate*10) / 10
	out.ContinuityPoints = math.Round(20*continuity*10) / 10
	out.LatencyPoints = math.Round(10*latency*10) / 10
	if successes == 0 {
		out.AvailabilityPoints = 0
		out.ContinuityPoints = 0
		out.LatencyPoints = 0
	}
	out.Score = &score
	out.Readiness = "provisional"
	if a.now-a.anchor >= out.WindowSeconds && out.Coverage >= .8 {
		out.Readiness = "ready"
	}
	return out
}
