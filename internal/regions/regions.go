package regions

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
)

type Match struct {
	Code   string `json:"code,omitempty"`
	Name   string `json:"name,omitempty"`
	Emoji  string `json:"emoji,omitempty"`
	Source string `json:"source"`
}

type Classifier struct {
	regions   []config.Region
	overrides map[string]string
}

func New(config config.Config) *Classifier {
	overrides := make(map[string]string, len(config.RegionOverrides))
	for name, code := range config.RegionOverrides {
		overrides[name] = code
	}
	return &Classifier{regions: config.Regions, overrides: overrides}
}

func (c *Classifier) Classify(nodeName string) Match {
	if code, ok := c.overrides[nodeName]; ok {
		if region, found := c.find(code); found {
			return toMatch(region, "manual")
		}
	}
	var best *config.Region
	bestLength := 0
	for index := range c.regions {
		region := &c.regions[index]
		for _, alias := range region.Aliases {
			if aliasMatch(nodeName, alias) && len([]rune(alias)) > bestLength {
				best = region
				bestLength = len([]rune(alias))
			}
		}
	}
	if best == nil {
		return Match{Source: "unknown"}
	}
	return toMatch(*best, "name-inferred")
}

func (c *Classifier) find(code string) (config.Region, bool) {
	for _, region := range c.regions {
		if region.Code == code {
			return region, true
		}
	}
	return config.Region{}, false
}

func toMatch(region config.Region, source string) Match {
	return Match{Code: region.Code, Name: region.Name, Emoji: region.Emoji, Source: source}
}

func aliasMatch(name, alias string) bool {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return false
	}
	if hasOnlyASCIIWordChars(alias) {
		if isTwoLetterCountryCode(alias) {
			// ISO-like codes are commonly immediately followed by a sequence
			// number and protocol suffix (JP4-HY2, KR1, US2TCP). Accept that
			// format without allowing the code inside a larger word such as SJP.
			pattern := `(?i)(^|[^A-Z0-9])` + regexp.QuoteMeta(alias) + `([0-9][A-Z0-9_-]*|[^A-Z0-9]|$)`
			return regexp.MustCompile(pattern).FindStringIndex(name) != nil
		}
		pattern := `(?i)(^|[^A-Z0-9])` + regexp.QuoteMeta(alias) + `($|[^A-Z0-9])`
		return regexp.MustCompile(pattern).FindStringIndex(name) != nil
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(alias))
}

func isTwoLetterCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func hasOnlyASCIIWordChars(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
