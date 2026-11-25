package probe

import (
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

func TestProfileHashMatchesStoredBaselineFingerprints(t *testing.T) {
	for _, tc := range []struct {
		probes []config.Probe
		want   string
	}{
		{nil, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
		{[]config.Probe{}, "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"},
		{[]config.Probe{{Name: "trace", URL: "https://example.com/health", ExpectedStatus: "200"}}, "b22078795f16b7637b0273ed9e4e2faf62477736cb869d2a3d724eb0dce4b47b"},
	} {
		if hash := ProfileHash(config.ProbeProfile{ID: "another-profile", Probes: tc.probes}); hash != tc.want {
			t.Errorf("stored fingerprint %s, got %s", tc.want, hash)
		}
	}
}
