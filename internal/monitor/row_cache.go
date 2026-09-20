package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type rowKey struct {
	series string
	window time.Duration
}
type rowEntry struct {
	version             history.EvidenceVersion
	anchor, first, last int64
	at, until           time.Time
	metrics             model.MonitorMetrics
	tail                []model.MonitorSample
	lastFailure         bool
	started             bool
}

func evidenceToken(v history.EvidenceVersion) string { return fmt.Sprintf("%d:%d", v.Epoch, v.Series) }

func (m *TaskRuntime) evidenceCurrent(rows []model.MonitorRow) bool {
	for _, row := range rows {
		if row.EvidenceVersion != evidenceToken(m.store.MonitorEvidenceVersion(row.SeriesID)) {
			return false
		}
	}
	return true
}

func (m *TaskRuntime) pruneRows(p model.MonitorPlan) {
	active := make(map[string]bool, len(p.Nodes))
	for _, n := range p.Nodes {
		active[n.SeriesID] = true
	}
	m.queries.mu.Lock()
	defer m.queries.mu.Unlock()
	for key := range m.queries.rows {
		if !active[key.series] {
			delete(m.queries.rows, key)
		}
	}
}

// Only metrics and the last 60 samples are retained, never an entire archive.
// Sliding slot boundaries invalidate the entry even with no incoming samples.
func (m *TaskRuntime) historyRow(ctx context.Context, series model.MonitorSeries, now time.Time, window time.Duration, cached bool) (model.MonitorRow, error) {
	version := m.store.MonitorEvidenceVersion(series.ID)
	key := rowKey{series.ID, window}
	first := max(int64(0), (max(series.Anchor, now.Add(-window).Unix())-series.Anchor+Interval-1)/Interval)
	last := (now.Unix() - series.Anchor) / Interval
	q := m.queries
	q.mu.Lock()
	entry, ok := q.rows[key]
	q.mu.Unlock()
	if !cached || !ok || entry.version != version || entry.anchor != series.Anchor || entry.started != (now.Unix() >= series.Anchor) || entry.first != first || entry.last != last || now.Before(entry.at) || !now.Before(entry.until) {
		samples, err := m.store.SeriesSamples(ctx, series, now.Add(-window), now, false)
		if err != nil {
			return model.MonitorRow{}, err
		}
		entry = rowEntry{version: version, anchor: series.Anchor, first: first, last: last, at: now, until: now.Add(120 * time.Second), metrics: windowMetrics(samples, series.Anchor, now, window), tail: append([]model.MonitorSample{}, samples[max(0, len(samples)-60):]...)}
		entry.started = now.Unix() >= series.Anchor
		if len(samples) > 0 {
			s := samples[len(samples)-1]
			entry.lastFailure = s.Slot == last && s.Outcome != "success" && s.Outcome != "unknown"
		}
		if cached && version == m.store.MonitorEvidenceVersion(series.ID) {
			q.mu.Lock()
			q.rows[key] = entry
			q.mu.Unlock()
		}
	}
	metrics := entry.metrics
	metrics.ObservedSeconds = min(int64(window/time.Second), max(int64(0), now.Unix()-series.Anchor))
	if entry.lastFailure {
		elapsed := func(at time.Time) int64 {
			return min(int64(Interval), max(int64(0), at.Unix()-(series.Anchor+last*Interval)))
		}
		metrics.FailureSeconds += int(elapsed(now) - elapsed(entry.at))
	}
	if metrics.Score != nil {
		score := *metrics.Score
		metrics.Score = &score
	}
	return model.MonitorRow{EvidenceVersion: evidenceToken(entry.version), Metrics: metrics, Series: append([]model.MonitorSample{}, entry.tail...)}, nil
}
