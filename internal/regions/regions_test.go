package regions

import (
	"testing"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
)

func TestClassifierPrefersManualOverrideAndAvoidsShortAliasSubstring(t *testing.T) {
	classifier := New(config.Config{
		Regions: []config.Region{
			{Code: "JP", Name: "Japan", Aliases: []string{"JP", "Tokyo", "日本"}},
			{Code: "US", Name: "United States", Aliases: []string{"US", "USA", "Los Angeles"}},
			{Code: "KR", Name: "Korea", Aliases: []string{"KR", "Korea", "韩国"}},
		},
		RegionOverrides: map[string]string{"IPLC-HY2-A-03": "JP"},
	})
	if got := classifier.Classify("IPLC-HY2-A-03"); got.Code != "JP" || got.Source != "manual" {
		t.Fatalf("manual match = %#v", got)
	}
	if got := classifier.Classify("US - Los Angeles 01"); got.Code != "US" || got.Source != "name-inferred" {
		t.Fatalf("US match = %#v", got)
	}
	if got := classifier.Classify("sushi-fast-01"); got.Code != "" {
		t.Fatalf("must not detect US inside ordinary text: %#v", got)
	}
	for _, sample := range []struct {
		name string
		code string
	}{
		{name: "JP4-HY2", code: "JP"},
		{name: "JP1-HY2", code: "JP"},
		{name: "JP-4", code: "JP"},
		{name: "KR1", code: "KR"},
		{name: "KR-1", code: "KR"},
	} {
		got := classifier.Classify(sample.name)
		if got.Code != sample.code {
			t.Fatalf("country-code sequence %q = %#v, want %s", sample.name, got, sample.code)
		}
	}
}
