package scan

import (
	"context"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func rankNodes(results []model.NodeResult) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if (a.Stage == "refined") != (b.Stage == "refined") {
			return a.Stage == "refined"
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.SuccessRate != b.SuccessRate {
			return a.SuccessRate > b.SuccessRate
		}
		return a.P95MS < b.P95MS
	})
	for i := range results {
		results[i].Rank = i + 1
	}
}

func shortlist(results []model.NodeResult, k int, current string) map[string]bool {
	ordered := append([]model.NodeResult(nil), results...)
	rankNodes(ordered)
	chosen := map[string]bool{}
	for _, r := range ordered {
		if len(chosen) >= k {
			break
		}
		if r.SuccessRate > 0 {
			chosen[r.Name] = true
		}
	}
	for _, r := range ordered {
		if r.Name == current {
			chosen[r.Name] = true
		}
	}
	return chosen
}

func (m *Manager) runBatches(ctx context.Context, s model.Scan, p config.ProbeProfile, candidates []candidate, stage string) ([]model.NodeResult, error) {
	cfg := m.currentConfig().Scanner
	results := make([]model.NodeResult, 0, len(candidates))
	for start := 0; start < len(candidates); start += cfg.BatchSize {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		progress := m.progressSnapshot(s.ID)
		progress.Stage = stage
		progress.CurrentBatch++
		progress.BatchCompleted = 0
		progress.BatchTotal = min(cfg.BatchSize, len(candidates)-start)
		m.setProgress(s.ID, progress)
		m.publish(s.ID, Event{Kind: "batch-started", Message: stage, At: time.Now().UTC(), Progress: &progress})
		results = append(results, m.probeBatch(ctx, s, p, candidates[start:start+progress.BatchTotal])...)
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if m.stopRequested(s.ID) {
			return results, errScanStopped
		}
	}
	return results, nil
}

func (m *Manager) stagedProbes(ctx context.Context, s model.Scan, p config.ProbeProfile, candidates []candidate, current string) ([]model.NodeResult, error) {
	cfg := m.currentConfig().Scanner
	m.setProgress(s.ID, model.ScanProgress{Total: len(candidates), TotalBatches: (len(candidates) + cfg.BatchSize - 1) / cfg.BatchSize, Stage: "screening"})
	first := s
	first.Request.Mode = "quick"
	results, err := m.runBatches(ctx, first, p, candidates, "screening")
	if err != nil || s.Request.Mode != "stable" {
		return results, err
	}
	chosen := shortlist(results, cfg.RefineTopK, current)
	for _, name := range s.Request.Nodes {
		chosen[name] = true
	}
	refined := make([]candidate, 0, len(chosen))
	for _, c := range candidates {
		if chosen[c.Name] {
			refined = append(refined, c)
		}
	}
	progress := m.progressSnapshot(s.ID)
	progress.Total += len(refined)
	progress.TotalBatches += (len(refined) + cfg.BatchSize - 1) / cfg.BatchSize
	progress.Stage = "refining"
	m.setProgress(s.ID, progress)
	next := s
	next.Request.Mode = "refine"
	extra, err := m.runBatches(ctx, next, p, refined, "refining")
	for _, r := range extra {
		for i := range results {
			if results[i].Name == r.Name {
				results[i].Samples = append(results[i].Samples, r.Samples...)
				results[i].MeasuredAt = r.MeasuredAt
				results[i].RefinementSamples = r.RefinementSamples
				results[i].Stage = "refined"
				calculateMetrics(&results[i], cfg)
				assessResult(&results[i], p, m.currentConfig().Scanner.StrictVerification.Enabled)
			}
		}
	}
	return results, err
}
