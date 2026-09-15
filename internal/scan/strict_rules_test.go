package scan

import (
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestStrictRulesIdentityIncludesHiddenAssertions(t *testing.T) {
	probe := config.StrictProbe{Name: "endpoint", URL: "https://example.com/private", ExpectedStatus: "200", BodyContains: "private-content"}
	id := strictRulesID([]config.StrictProbe{probe})
	if id == "" || id != strictRulesID([]config.StrictProbe{probe}) || strings.Contains(id, "private") {
		t.Fatal("unstable or revealing strict rule identity")
	}
	for _, field := range []string{"body", "url", "status", "restriction"} {
		changed := probe
		switch field {
		case "body":
			changed.BodyContains = "changed"
		case "url":
			changed.URL = "https://example.com/other"
		case "status":
			changed.ExpectedStatus = "204"
		case "restriction":
			changed.RestrictedStatusCodes = []int{403}
		}
		if strictRulesID([]config.StrictProbe{changed}) == id {
			t.Errorf("%s rule change retained old identity", field)
		}
	}
	if strictRulesID(nil) != "" {
		t.Fatal("missing rules must have no identity")
	}
}
