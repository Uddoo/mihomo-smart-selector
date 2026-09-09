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
	out := model.MonitorMetrics{Readiness: "collecting", WindowSeconds: int64(window / time.Second), ObservedSeconds: min(int64(window/time.Second), max(int64(0), now.Unix()-anchor))}
	start := max(anchor, now.Add(-window).Unix())
	first := max(int64(0), (start-anchor+Interval-1)/Interval)
	last := (now.Unix() - anchor) / Interval
	if now.Unix() >= anchor && last >= first {
		out.Expected = int(last - first + 1)
	}
	delays := []int{}
	successes := 0
	previousSlot := int64(-2)
	failed := false
	baseline := make([]model.MonitorSample, 0, len(samples))
	for _, s := range samples {
		if s.Kind == "baseline" && s.Slot >= first && s.Slot <= last {
			baseline = append(baseline, s)
		}
	}
	sort.SliceStable(baseline, func(i, j int) bool { return baseline[i].Slot < baseline[j].Slot })
	seen := make(map[int64]bool, len(baseline))
	for _, s := range baseline {
		if seen[s.Slot] {
			continue
		}
		seen[s.Slot] = true
		if s.Outcome == "unknown" {
			failed = false
			previousSlot = s.Slot
			continue
		}
		out.Samples++
		if s.Outcome == "success" {
			successes++
			delays = append(delays, s.DelayMS)
			failed = false
		} else {
			out.FailureSeconds += int(min(int64(Interval), max(int64(0), now.Unix()-(anchor+s.Slot*Interval))))
			if !failed || s.Slot != previousSlot+1 {
				out.Incidents++
			}
			failed = true
		}
		previousSlot = s.Slot
	}
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
	if window <= time.Hour {
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
	if now.Unix()-anchor >= int64(window/time.Second) && out.Coverage >= .8 {
		out.Readiness = "ready"
	}
	return out
}
