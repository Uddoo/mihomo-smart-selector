package regions

import (
	"reflect"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestBuiltinCoverageAndNormalization(t *testing.T) {
	c := New(config.Config{Regions: []config.Region{{Code: "JP", Name: "Japan", Aliases: []string{"JP"}}}})
	for _, sample := range []struct{ name, code string }{
		{"[2x]香港 01", "HK"}, {"新加坡02", "SG"}, {"TW-10", "TW"},
		{"🇦🇪13迪拜-专线", "AE"}, {"🇦🇺 悉尼 · 01", "AU"}, {"法兰克福 02", "DE"},
		{"ｈｋ３－ＨＹ２", "HK"}, {"ＪＰ４", "JP"}, {"台灣-02", "TW"},
		{"印度尼西亚 01", "ID"}, {"🇸🇬", "SG"}, {"in3", "IN"}, {"no-1", "NO"},
	} {
		if got := c.Classify(sample.name); got.Code != sample.code || len(got.Evidence) == 0 {
			t.Errorf("%q: %#v, want %s", sample.name, got, sample.code)
		}
	}
	for _, name := range []string{"sushi-fast", "SJP1", "shopping", "no connection", "made in heaven", "my favorite", "🇿🇿", "plain-node"} {
		if got := c.Classify(name); got.Code != "" || got.Source != "unknown" {
			t.Errorf("false positive %q: %#v", name, got)
		}
	}
}

func TestConflictsTransitAndManualPrecedence(t *testing.T) {
	c := New(config.Config{RegionOverrides: map[string]string{"香港→日本": "jp"}})
	for _, sample := range []struct {
		name, reason string
		codes        []string
	}{
		{"🇯🇵 香港", "conflicting-cues", []string{"HK", "JP"}},
		{"SG / TW", "conflicting-cues", []string{"SG", "TW"}},
		{"LA香港中转01", "transit", []string{"HK"}},
		{"Japan via Singapore", "transit", []string{"JP", "SG"}},
	} {
		got := c.Classify(sample.name)
		if got.Code != "" || got.Source != "ambiguous" || got.Reason != sample.reason || !reflect.DeepEqual(got.Candidates, sample.codes) {
			t.Errorf("%q: %#v", sample.name, got)
		}
	}
	if got := c.Classify("香港→日本"); got.Code != "JP" || got.Source != "manual" {
		t.Fatalf("manual precedence: %#v", got)
	}
	a := New(config.Config{Regions: []config.Region{{Code: "SG", Aliases: []string{"Tokyo"}}}})
	if got := a.Classify("Tokyo"); got.Source != "ambiguous" {
		t.Fatalf("custom conflicting alias silently won: %#v", got)
	}
}

func TestEntryClassificationPreservesRealNodes(t *testing.T) {
	c := New(config.Config{})
	for _, sample := range []struct{ name, protocol, kind, source string }{
		{"DIRECT", "Direct", "builtin", "not-applicable"},
		{"JP Reject", "Reject-Drop", "builtin", "not-applicable"},
		{"DIRECT", "Vless", "proxy", "unknown"},
		{"过期时间：2030-01-01", "Shadowsocks", "subscription-info", "not-applicable"},
		{"v0-网址:https://example.test", "Vmess", "subscription-info", "not-applicable"},
		{"🎁 邀请好友得奖励", "Vless", "subscription-info", "not-applicable"},
		{"🌏自动最优线路-网址:example.test", "AnyTLS", "dynamic", "dynamic"},
		{"香港 01 | 官网:example.test", "Vless", "proxy", "name-inferred"},
	} {
		kind, match := c.ClassifyEntry(sample.name, sample.protocol)
		if kind != sample.kind || match.Source != sample.source {
			t.Errorf("%q: %s %#v", sample.name, kind, match)
		}
	}
}

func TestManualOverrideCanCorrectSuspectedNotice(t *testing.T) {
	c := New(config.Config{RegionOverrides: map[string]string{"网址：专用线路": "HK", "🌏自动最优线路": "JP", "DIRECT": "HK"}})
	kind, match := c.ClassifyEntry("网址：专用线路", "Vless")
	if kind != "proxy" || match.Code != "HK" || match.Source != "manual" {
		t.Fatalf("explicit node metadata must correct a heuristic: %s %#v", kind, match)
	}
	kind, match = c.ClassifyEntry("🌏自动最优线路", "AnyTLS")
	if kind != "dynamic" || match.Code != "JP" || match.Source != "manual" {
		t.Fatalf("dynamic override: %s %#v", kind, match)
	}
	kind, match = c.ClassifyEntry("DIRECT", "Direct")
	if kind != "builtin" || match.Source != "not-applicable" {
		t.Fatalf("built-in outbound is not a regional proxy: %s %#v", kind, match)
	}
}

func BenchmarkClassifier(b *testing.B) {
	c := New(config.Config{})
	for b.Loop() {
		c.Classify("🇭🇰 香港 · HKBN-01|直连")
	}
}

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
