package scan

import (
	"testing"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
	"github.com/yw-li/mihomo-smart-selector/internal/model"
)

func TestCalculateMetricsFavorsStableSuccessfulNode(t *testing.T) {
	cfg := config.ScannerConfig{MedianTargetMS: 300, P95TargetMS: 800, JitterTargetMS: 200}
	stable := model.NodeResult{
		InferredRegion: "JP", VerifiedRegion: "JP",
		Samples: []model.ProbeSample{
			{Probe: "trace", DelayMS: 120}, {Probe: "trace", DelayMS: 130}, {Probe: "trace", DelayMS: 125},
			{Probe: "api", DelayMS: 160}, {Probe: "api", DelayMS: 165}, {Probe: "api", DelayMS: 162},
		},
	}
	unstable := model.NodeResult{
		Samples: []model.ProbeSample{
			{Probe: "trace", DelayMS: 80}, {Probe: "trace", Error: "timeout"}, {Probe: "trace", DelayMS: 850},
			{Probe: "api", DelayMS: 70}, {Probe: "api", Error: "timeout"}, {Probe: "api", DelayMS: 1100},
		},
	}
	calculateMetrics(&stable, cfg)
	calculateMetrics(&unstable, cfg)
	if stable.SuccessRate != 1 || stable.P50MS != 130 || stable.P95MS != 165 {
		t.Fatalf("stable metrics = %#v", stable)
	}
	if stable.Score <= unstable.Score {
		t.Fatalf("stable score %v must exceed unstable score %v", stable.Score, unstable.Score)
	}
	if stable.Score != stable.ScoreBreakdown.Total || stable.ScoreBreakdown.Reliability <= 0 || stable.ScoreBreakdown.Region != 5 {
		t.Fatalf("stable score breakdown = %#v", stable.ScoreBreakdown)
	}
}
