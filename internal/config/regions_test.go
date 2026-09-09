package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyRegionsExtendBuiltinsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "regions:\n  - code: jp\n    name: Custom Japan\n    aliases: [JP, PrivateTokyo]\nregion_overrides:\n  opaque: hk\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]Region{}
	for _, region := range cfg.Regions {
		byCode[region.Code] = region
	}
	if len(byCode) != 40 || byCode["JP"].Name != "Custom Japan" || byCode["HK"].Name != "香港" || byCode["AE"].Name != "阿联酋" {
		t.Fatalf("effective catalog: %#v", byCode)
	}
	aliases := map[string]bool{}
	for _, alias := range byCode["JP"].Aliases {
		aliases[alias] = true
	}
	if !aliases["PrivateTokyo"] || !aliases["日本"] || !aliases["🇯🇵"] {
		t.Fatalf("aliases: %#v", aliases)
	}
	if !reflect.DeepEqual(cfg.Regions, MergeRegions(cfg.Regions)) {
		t.Fatal("merge must be idempotent")
	}
}

func TestRegionMergeIsolationAndValidation(t *testing.T) {
	custom := []Region{{Code: "ZZ", Name: "Private region", Aliases: []string{"PrivateRegion"}}}
	first := MergeRegions(custom)
	first[0].Aliases[0] = "modified"
	first[len(first)-1].Aliases[0] = "modified"
	if custom[0].Aliases[0] != "PrivateRegion" || BuiltinRegions()[0].Aliases[0] != "JP" {
		t.Fatal("merge mutated input or defaults")
	}
	for _, entries := range [][]Region{
		{{Code: "JP", Name: "one", Aliases: []string{"a"}}, {Code: "jp", Name: "two", Aliases: []string{"b"}}},
		{{Code: "ZZ", Name: "missing aliases"}},
		{{Code: "", Name: "empty code", Aliases: []string{"x"}}},
	} {
		cfg := Defaults()
		cfg.Regions = entries
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected invalid region config: %#v", entries)
		}
	}
}
