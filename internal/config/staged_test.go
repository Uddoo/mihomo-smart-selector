package config

import "testing"

func TestStagedSettingsAndServiceGateValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
	}{
		{"zero K", func(c *Config) { c.Scanner.RefineTopK = 0 }},
		{"short expiry", func(c *Config) { c.Scanner.ResultMaxAgeSeconds = 29 }},
		{"strict without probes", func(c *Config) { c.Scanner.ProbeProfiles[0].RequireStrict = true }},
		{"region without countries", func(c *Config) { c.Scanner.ProbeProfiles[0].RequireRegion = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Defaults()
			tc.mutate(&c)
			if c.Validate() == nil {
				t.Fatal("invalid settings accepted")
			}
		})
	}
	c := Defaults()
	yes := true
	no := false
	c.Scanner.ProbeProfileOverrides = map[string]ProbeProfileOverride{"chatgpt": {RequireStrict: &yes, RequireRegion: &yes, ExpectedRegions: []string{"JP"}}}
	if err := c.applyProbeProfileOverrides(); err != nil {
		t.Fatal(err)
	}
	p, err := c.ProbeProfileByID("chatgpt")
	if err != nil || !p.RequireStrict || !p.RequireRegion {
		t.Fatal("policy override not applied")
	}
	c.Scanner.ProbeProfileOverrides = map[string]ProbeProfileOverride{"chatgpt": {RequireStrict: &no, RequireRegion: &no}}
	if err := c.applyProbeProfileOverrides(); err != nil {
		t.Fatal(err)
	}
	p, _ = c.ProbeProfileByID("chatgpt")
	if p.RequireStrict || p.RequireRegion {
		t.Fatal("explicit false ignored")
	}
}
