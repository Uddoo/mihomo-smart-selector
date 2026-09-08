package scan

import (
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestJitterScoreDistinguishesZeroFromInsufficientSamples(t *testing.T) {
	cfg := config.ScannerConfig{MedianTargetMS: 300, P95TargetMS: 800, JitterTargetMS: 200}
	for _, tc := range []struct {
		name    string
		samples []model.ProbeSample
		want    float64
	}{
		{"zero", []model.ProbeSample{{Probe: "a", DelayMS: 100}, {Probe: "a", DelayMS: 100}}, 10},
		{"small", []model.ProbeSample{{Probe: "a", DelayMS: 100}, {Probe: "a", DelayMS: 102}}, 9.9},
		{"single", []model.ProbeSample{{Probe: "a", DelayMS: 100}}, 0},
		{"different probes", []model.ProbeSample{{Probe: "a", DelayMS: 100}, {Probe: "b", DelayMS: 100}}, 0},
		{"failed second sample", []model.ProbeSample{{Probe: "a", DelayMS: 100}, {Probe: "a", Error: "timeout"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := model.NodeResult{Samples: tc.samples}
			calculateMetrics(&result, cfg)
			if result.ScoreBreakdown.Jitter != tc.want {
				t.Fatalf("jitter points = %v, want %v", result.ScoreBreakdown.Jitter, tc.want)
			}
		})
	}
}

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
