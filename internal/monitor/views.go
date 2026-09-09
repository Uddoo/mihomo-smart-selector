package monitor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func WindowDuration(window string) (time.Duration, error) {
	switch window {
	case "1h":
		return time.Hour, nil
	case "", "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("window 必须为 1h、24h 或 7d")
	}
}
func (m *Manager) Overview(ctx context.Context) (model.MonitorOverview, error) {
	return m.overview(ctx, "24h", true)
}
func (m *Manager) WindowOverview(ctx context.Context, window string) (model.MonitorOverview, error) {
	return m.cachedOverview(ctx, window)
}

func (m *Manager) overview(ctx context.Context, window string, details bool) (model.MonitorOverview, error) {
	duration, err := WindowDuration(window)
	if err != nil {
		return model.MonitorOverview{}, err
	}
	if window == "" {
		window = "24h"
	}
	for attempt := 0; attempt < 3; attempt++ {
		m.mu.Lock()
		now := m.now()
		o := model.MonitorOverview{Plan: clonePlan(m.plan), Current: m.current, Issue: m.issue, Suspended: m.fault, FailoverMessage: m.failoverMessage, ObservedAt: m.observedAt, Now: now, Rows: []model.MonitorRow{}, Events: []model.MonitorEvent{}, RetentionDays: 7, Window: window, DataVersion: m.dataVersion, InstanceID: m.instanceID}
		states := map[string]model.MonitorState{}
		for k, v := range m.states {
			states[k] = v
		}
		m.mu.Unlock()
		if o.Plan == nil {
			return o, nil
		}
		p := *o.Plan
		if details {
			policy, e := m.store.MonitorRetention(ctx, m.source.MonitorScope())
			if e != nil {
				return o, e
			}
			o.RetentionDays = policy.RawDays
			o.Events, err = m.store.MonitorEvents(ctx, p.ID)
			if err != nil {
				return o, err
			}
		}
		for i, n := range p.Nodes {
			series := model.MonitorSeries{ID: n.SeriesID, Node: n, ProfileID: p.ProfileID, ProfileHash: p.ProfileHash, Anchor: nodeAnchor(p, i)}
			samples, e := m.store.SeriesSamples(ctx, series, now.Add(-duration), now, false)
			if e != nil {
				return o, e
			}
			row := model.MonitorRow{MonitorNode: n, State: states[n.ID], Metrics: windowMetrics(samples, series.Anchor, now, duration), Series: []model.MonitorSample{}}
			if details {
				row.Series = append([]model.MonitorSample{}, samples[max(0, len(samples)-60):]...)
			}
			if !p.Enabled || o.Suspended || o.Issue != "" || !fresh(row.State.LastAt, now, 2*Interval*time.Second) {
				row.State.Status = "unknown"
			}
			o.Rows = append(o.Rows, row)
			anchor := series.Anchor
			next := anchor
			if now.Unix() >= anchor {
				next = anchor + ((now.Unix()-anchor)/Interval+1)*Interval
			}
			t := time.Unix(next, 0).UTC()
			if o.NextAt.IsZero() || t.Before(o.NextAt) {
				o.NextAt = t
			}
		}
		if !p.Enabled || o.Suspended {
			o.NextAt = time.Time{}
		}
		sort.SliceStable(o.Rows, func(i, j int) bool {
			a, b := o.Rows[i], o.Rows[j]
			if (a.State.Status == "healthy") != (b.State.Status == "healthy") {
				return a.State.Status == "healthy"
			}
			if (a.Metrics.Readiness == "ready") != (b.Metrics.Readiness == "ready") {
				return a.Metrics.Readiness == "ready"
			}
			if a.Metrics.Score != nil && b.Metrics.Score != nil && *a.Metrics.Score != *b.Metrics.Score {
				return *a.Metrics.Score > *b.Metrics.Score
			}
			if a.Metrics.SuccessRate != b.Metrics.SuccessRate {
				return a.Metrics.SuccessRate > b.Metrics.SuccessRate
			}
			return a.Metrics.P95MS < b.Metrics.P95MS
		})
		m.mu.Lock()
		valid := o.DataVersion == m.dataVersion
		m.mu.Unlock()
		if valid {
			return o, nil
		}
	}
	return model.MonitorOverview{}, fmt.Errorf("监控数据正在更新，请稍后重试")
}

func trends(samples []model.MonitorSample, anchor int64, from, to time.Time) []model.MonitorTrend {
	out := []model.MonitorTrend{}
	bucket := int64(3600)
	if to.Sub(from) <= time.Hour {
		bucket = 120
	}
	by := map[int64][]model.MonitorSample{}
	for _, v := range samples {
		if v.Kind == "baseline" {
			hour := v.ScheduledAt / bucket * bucket
			by[hour] = append(by[hour], v)
		}
	}
	for h := from.Unix() / bucket * bucket; h <= to.Unix()/bucket*bucket; h += bucket {
		left := max(h, from.Unix(), anchor)
		right := min(h+bucket-1, to.Unix())
		expected := 0
		if right >= left {
			first := max(int64(0), (left-anchor+Interval-1)/Interval)
			last := (right - anchor) / Interval
			if last >= first {
				expected = int(last - first + 1)
			}
		}
		point := model.MonitorTrend{At: time.Unix(h, 0).UTC(), Expected: expected}
		delays := []int{}
		for _, v := range by[h] {
			switch v.Outcome {
			case "success":
				point.Success++
				delays = append(delays, v.DelayMS)
			case "failure":
				point.Failure++
			}
		}
		point.Unknown = max(0, expected-point.Success-point.Failure)
		if len(delays) > 0 {
			sort.Ints(delays)
			p50 := delays[int(math.Ceil(.5*float64(len(delays))))-1]
			p95 := delays[int(math.Ceil(.95*float64(len(delays))))-1]
			point.P50 = &p50
			point.P95 = &p95
		}
		out = append(out, point)
	}
	return out
}

func (m *Manager) Timeline(ctx context.Context, id string, from, to time.Time) (model.MonitorTimeline, error) {
	release, err := m.acquireHistory(ctx)
	if err != nil {
		return model.MonitorTimeline{}, err
	}
	defer release()
	now := m.now()
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.Add(-7 * 24 * time.Hour)
	}
	var out model.MonitorTimeline
	if !to.After(from) || to.After(now.Add(time.Minute)) || to.Sub(from) > 90*24*time.Hour {
		return out, fmt.Errorf("时间范围长度不能超过 90 天，结束时间不能在未来")
	}
	series, err := m.store.SeriesDefinition(ctx, m.source.MonitorScope(), id)
	if err != nil {
		return out, err
	}
	samples, err := m.store.SeriesSamples(ctx, series, from, to, false)
	if err != nil {
		return out, err
	}
	out = model.MonitorTimeline{Series: series, From: from, To: to, Metrics: windowMetrics(samples, series.Anchor, to, to.Sub(from)), Trend: trends(samples, series.Anchor, from, to)}
	m.mu.Lock()
	if m.plan != nil {
		for _, n := range m.plan.Nodes {
			if n.SeriesID == id {
				out.Active = true
			}
		}
	}
	m.mu.Unlock()
	return out, nil
}

func (m *Manager) Series(ctx context.Context) ([]model.MonitorSeries, error) {
	return m.store.MonitorSeries(ctx, m.source.MonitorScope())
}
func (m *Manager) Revisions(ctx context.Context) ([]model.MonitorRevision, error) {
	return m.store.MonitorRevisions(ctx, m.source.MonitorScope())
}
