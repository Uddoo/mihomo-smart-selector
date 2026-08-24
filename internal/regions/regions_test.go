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
}
