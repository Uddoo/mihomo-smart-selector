package scan

import (
	"math"
	"sort"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
	"github.com/yw-li/mihomo-smart-selector/internal/model"
)

func calculateMetrics(result *model.NodeResult, cfg config.ScannerConfig) {
	delays := make([]int, 0, len(result.Samples))
	byProbe := map[string][]int{}
	for _, sample := range result.Samples {
		if sample.Error == "" && sample.DelayMS > 0 {
			delays = append(delays, sample.DelayMS)
			byProbe[sample.Probe] = append(byProbe[sample.Probe], sample.DelayMS)
		}
	}
	if len(result.Samples) > 0 {
		result.SuccessRate = float64(len(delays)) / float64(len(result.Samples))
	}
	if len(delays) == 0 {
		result.Score = 0
		result.ScoreBreakdown = model.ScoreBreakdown{}
		return
	}
	sort.Ints(delays)
	result.P50MS = nearestRank(delays, 0.50)
	result.P95MS = nearestRank(delays, 0.95)
	var jitterSum float64
	var jitterCount int
	for _, values := range byProbe {
		for index := 1; index < len(values); index++ {
			jitterSum += math.Abs(float64(values[index] - values[index-1]))
			jitterCount++
		}
	}
	if jitterCount > 0 {
		result.JitterMS = jitterSum / float64(jitterCount)
	}

	reliability := 40 * result.SuccessRate
	p95 := scaledComponent(result.P95MS, cfg.P95TargetMS, 20)
	p50 := scaledComponent(result.P50MS, cfg.MedianTargetMS, 15)
	jitter := scaledComponent(int(math.Round(result.JitterMS)), cfg.JitterTargetMS, 10)
	region := 0.0
	if result.VerifiedRegion != "" && result.VerifiedRegion == result.InferredRegion {
		region = 5
	}
	result.ScoreBreakdown = model.ScoreBreakdown{
		Reliability: round1(reliability), P50: round1(p50), P95: round1(p95),
		Jitter: round1(jitter), Region: round1(region), Total: round1(reliability + p50 + p95 + jitter + region),
	}
	result.Score = result.ScoreBreakdown.Total
}

func scaledComponent(measurement, target, maximum int) float64 {
	if measurement <= 0 || target <= 0 {
		return 0
	}
	fraction := 1 - float64(measurement)/float64(target)
	if fraction < 0 {
		fraction = 0
	}
	return float64(maximum) * fraction
}

func nearestRank(sorted []int, percentile float64) int {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}
