package regions

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

type Match struct {
	Code       string   `json:"code,omitempty"`
	Name       string   `json:"name,omitempty"`
	Emoji      string   `json:"emoji,omitempty"`
	Source     string   `json:"source"`
	Reason     string   `json:"reason,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`
}

type aliasRule struct {
	code, alias string
	pattern     *regexp.Regexp
}
type hit struct {
	code, alias string
	start, end  int
}
type Classifier struct {
	regions   []config.Region
	overrides map[string]string
	rules     []aliasRule
}

func New(cfg config.Config) *Classifier {
	c := &Classifier{regions: config.MergeRegions(cfg.Regions), overrides: map[string]string{}}
	for name, code := range cfg.RegionOverrides {
		c.overrides[name] = strings.ToUpper(strings.TrimSpace(code))
	}
	for _, region := range c.regions {
		for _, alias := range region.Aliases {
			alias = normalizeName(strings.TrimSpace(alias))
			if alias == "" {
				continue
			}
			pattern := "(?i)(" + regexp.QuoteMeta(alias) + ")"
			if asciiWord(alias) {
				suffix := "(?:$|[^A-Z0-9])"
				if len(alias) == 2 {
					suffix = "(?:[0-9][A-Z0-9_-]*|$|[^A-Z0-9])"
				}
				pattern = "(?i)(?:^|[^A-Z0-9])(" + regexp.QuoteMeta(alias) + ")" + suffix
			}
			c.rules = append(c.rules, aliasRule{region.Code, alias, regexp.MustCompile(pattern)})
		}
	}
	return c
}

// Regions returns an isolated copy of the same effective dictionary used by
// classification, including built-ins when the caller supplies a legacy config.
func (c *Classifier) Regions() []config.Region {
	result := append([]config.Region(nil), c.regions...)
	for i := range result {
		result[i].Aliases = append([]string(nil), result[i].Aliases...)
	}
	return result
}

func (c *Classifier) Classify(nodeName string) Match {
	if code, ok := c.overrides[nodeName]; ok {
		if region, found := c.find(code); found {
			return toMatch(region, "manual")
		}
	}
	name := normalizeName(nodeName)
	var hits []hit
	for _, rule := range c.rules {
		for _, span := range rule.pattern.FindAllStringSubmatchIndex(name, -1) {
			start, end := span[2], span[3]
			// Lowercase English words such as "in", "it" and "no" are not country
			// labels unless attached to a sequence number. HK/jp/sg remain insensitive.
			if commonWordCode(rule.alias) && name[start:end] != strings.ToUpper(name[start:end]) && !numbered(name[end:]) {
				continue
			}
			hits = append(hits, hit{rule.code, rule.alias, start, end})
		}
	}
	// A longer place name can contain a shorter country name (印度尼西亚/印度).
	// Discard only contained shorter hits; independent conflicting cues survive.
	evidence := map[string]string{}
	for _, candidate := range hits {
		covered := false
		for _, other := range hits {
			if other.start <= candidate.start && other.end >= candidate.end && other.end-other.start > candidate.end-candidate.start {
				covered = true
				break
			}
		}
		if !covered && len(candidate.alias) > len(evidence[candidate.code]) {
			evidence[candidate.code] = candidate.alias
		}
	}
	codes := make([]string, 0, len(evidence))
	for code := range evidence {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	aliases := make([]string, 0, len(codes))
	for _, code := range codes {
		aliases = append(aliases, evidence[code])
	}
	if len(codes) == 0 {
		return Match{Source: "unknown"}
	}
	if transitName.MatchString(name) {
		return Match{Source: "ambiguous", Reason: "transit", Candidates: codes, Evidence: aliases}
	}
	if len(codes) > 1 {
		return Match{Source: "ambiguous", Reason: "conflicting-cues", Candidates: codes, Evidence: aliases}
	}
	region, _ := c.find(codes[0])
	result := toMatch(region, "name-inferred")
	result.Evidence = aliases
	return result
}

var transitName = regexp.MustCompile(`(?i)中转|中轉|转接|轉接|(?:^|[^A-Z0-9])(?:transit|via)(?:$|[^A-Z0-9])|→|->`)
var infoName = regexp.MustCompile(`(?i)^(?:v[0-9]+[-_])?(?:网址|網址|官网|官網|官方网站|官方網站|剩余流量|剩餘流量|套餐到期|过期时间|過期時間|到期时间|到期時間|邀请好友|邀請好友|流量重置|https?://)`)
var dynamicName = regexp.MustCompile(`^(?:自动最优|自動最優|自动选择|自動選擇|自动切换|自動切換|自动线路|自動線路)`)

// EntryKind is intentionally conservative: protocol identifies built-ins;
// anchored name heuristics identify only suspected subscription notices.
func EntryKind(name, protocol string) string {
	if IsBuiltin(protocol) {
		return "builtin"
	}
	plain := strings.TrimLeftFunc(normalizeName(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	if dynamicName.MatchString(plain) {
		return "dynamic"
	}
	if infoName.MatchString(plain) {
		return "subscription-info"
	}
	return "proxy"
}

func IsBuiltin(protocol string) bool {
	switch strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(protocol)) {
	case "direct", "reject", "rejectdrop", "pass", "passrule", "compatible":
		return true
	}
	return false
}

func (c *Classifier) ClassifyEntry(name, protocol string) (string, Match) {
	kind := EntryKind(name, protocol)
	if kind == "builtin" {
		return kind, Match{Source: "not-applicable"}
	}
	// An explicit per-node override can correct a notice-name false positive.
	if code, ok := c.overrides[name]; ok {
		if region, found := c.find(code); found {
			if kind == "subscription-info" {
				kind = "proxy"
			}
			return kind, toMatch(region, "manual")
		}
	}
	if kind == "subscription-info" {
		return kind, Match{Source: "not-applicable"}
	}
	if kind == "dynamic" {
		return kind, Match{Source: "dynamic"}
	}
	return kind, c.Classify(name)
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
func normalizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0xff01 && r <= 0xff5e {
			return r - 0xfee0
		}
		if r == 0x3000 || r == 0x00a0 {
			return ' '
		}
		if r == 0x200b || r == 0xfeff {
			return -1
		}
		return r
	}, name)
}
func asciiWord(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
func commonWordCode(value string) bool {
	switch strings.ToLower(value) {
	case "in", "it", "no", "at", "be", "ch", "id", "my", "is", "to", "as":
		return true
	}
	return false
}
func numbered(suffix string) bool {
	suffix = strings.TrimLeft(suffix, "-_")
	return len(suffix) > 0 && suffix[0] >= '0' && suffix[0] <= '9'
}
