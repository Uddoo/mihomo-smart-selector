package scan

import (
	"context"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"time"
)

func (m *Manager) setActive(scan model.Scan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := cloneScan(scan)
	m.active[scan.ID] = &snapshot
	m.indexResults(snapshot)
}

func (m *Manager) removeActive(scanID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, scanID)
	delete(m.resultIndex, scanID)
	delete(m.cancels, scanID)
	delete(m.stopAfterBatch, scanID)
}

func (m *Manager) setActiveWithCancel(scan model.Scan, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := cloneScan(scan)
	m.active[scan.ID] = &snapshot
	m.indexResults(snapshot)
	m.cancels[scan.ID] = cancel
}

func (m *Manager) setProgress(scanID string, progress model.ScanProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		active.Progress = progress
	}
}

func (m *Manager) progressCompleted(scanID string) int {
	return m.progressSnapshot(scanID).Completed
}

func (m *Manager) progressSnapshot(scanID string) model.ScanProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		return active.Progress
	}
	return model.ScanProgress{}
}

func (m *Manager) recordCandidate(scanID string, result model.NodeResult, profile config.ProbeProfile) (model.NodeResult, model.ScanProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		replaced := false
		if m.resultIndex[scanID] == nil {
			m.indexResults(*active)
		}
		if i, ok := m.resultIndex[scanID][result.Name]; ok {
			result.ScreeningSamples = active.Results[i].ScreeningSamples
			result.Samples = append(append([]model.ProbeSample(nil), active.Results[i].Samples...), result.Samples...)
			calculateMetrics(&result, m.currentConfig().Scanner)
			assessResult(&result, profile, m.currentConfig().Scanner.StrictVerification.Enabled)
			active.Results[i] = result
			replaced = true
		}
		if !replaced {
			m.resultIndex[scanID][result.Name] = len(active.Results)
			active.Results = append(active.Results, result)
		}
		active.Progress.Completed++
		active.Progress.BatchCompleted++
		if result.SuccessRate > 0 {
			active.Progress.Succeeded++
		} else {
			active.Progress.Failed++
		}
		active.Progress.ElapsedSeconds = int(time.Since(active.StartedAt).Seconds())
		if active.Progress.Completed > 0 {
			remaining := active.Progress.Total - active.Progress.Completed
			active.Progress.EstimatedRemainingSeconds = active.Progress.ElapsedSeconds * remaining / active.Progress.Completed
		}
		return result, active.Progress
	}
	return result, model.ScanProgress{}
}

// Caller holds m.mu. Rebuild only when replacing the complete scan snapshot.
func (m *Manager) indexResults(scan model.Scan) {
	if m.resultIndex == nil {
		m.resultIndex = map[string]map[string]int{}
	}
	index := make(map[string]int, len(scan.Results))
	for i, result := range scan.Results {
		if _, exists := index[result.Name]; !exists {
			index[result.Name] = i
		}
	}
	m.resultIndex[scan.ID] = index
}

func (m *Manager) stopRequested(scanID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAfterBatch[scanID]
}

func cloneScan(input model.Scan) model.Scan {
	output := input
	output.Warnings = append([]model.ScanWarning(nil), input.Warnings...)
	output.Request.Nodes = append([]string(nil), input.Request.Nodes...)
	output.Request.Regions = append([]string(nil), input.Request.Regions...)
	output.Request.Providers = append([]string(nil), input.Request.Providers...)
	output.Results = make([]model.NodeResult, len(input.Results))
	for index, result := range input.Results {
		output.Results[index] = result
		output.Results[index].Samples = append([]model.ProbeSample(nil), result.Samples...)
	}
	return output
}
