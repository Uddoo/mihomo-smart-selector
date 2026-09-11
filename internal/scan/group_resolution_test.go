package scan

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestNestedSelectorPreflightIsClientIndependentAndDoesNotFlatten(t *testing.T) {
	for _, root := range []string{"OpenAI", "OpenClash 服务策略", "work/custom-name"} {
		t.Run(root, func(t *testing.T) {
			fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{
				root:          {Name: root, Type: "Selector", Now: "shared", All: []string{"auto", "shared"}},
				"shared":      {Name: "shared", Type: "Selector", Now: "deeper", All: []string{"deeper", root}},
				"deeper":      {Name: "deeper", Type: "Selector", Now: "JP-01", All: []string{"JP-01", "JP-03", "DIRECT", "other-group"}},
				"JP-01":       {Name: "JP-01", Type: "VLESS"},
				"auto":        {Name: "auto", Type: "URLTest", All: []string{"JP-01"}},
				"DIRECT":      {Name: "DIRECT", Type: "Direct"},
				"other-group": {Name: "other-group", Type: "FuturePolicy", All: []string{"JP-01"}},
			}}
			m := NewManager(config.Defaults(), fake, nil)
			request := model.ScanRequest{TargetGroup: root, ProfileID: "chatgpt", Mode: "quick"}
			preview, err := m.Preflight(context.Background(), request)
			if err != nil || preview.Ready || preview.Profile.ID != "chatgpt" || len(preview.NestedSelectors) != 1 {
				t.Fatalf("preview=%+v err=%v", preview, err)
			}
			option := preview.NestedSelectors[0]
			if option.Group != "deeper" || option.CandidateCount != 2 || !option.OnCurrentPath || !reflect.DeepEqual(option.Path, []string{root, "shared", "deeper"}) {
				t.Fatalf("wrong suggested target: %+v", option)
			}
			if _, err := m.discoverCandidates(context.Background(), request); !errors.Is(err, errNoCandidates) {
				t.Fatalf("parent membership was flattened: %v", err)
			}
			request.TargetGroup = option.Group
			ready, err := m.Preflight(context.Background(), request)
			if err != nil || !ready.Ready || ready.CandidateCount != 2 || ready.Profile.ID != preview.Profile.ID {
				t.Fatalf("explicit child target not scannable: %+v, %v", ready, err)
			}
			if fake.selected != "" || fake.proxies[root].Now != "shared" {
				t.Fatal("read-only preflight changed the controller")
			}
		})
	}
}

func TestNestedSelectorCyclesSharedPathsAndUnselectablePolicies(t *testing.T) {
	m := NewManager(config.Defaults(), &fakeMihomo{}, nil)
	proxies := map[string]mihomo.Proxy{
		"root":       {Name: "root", Type: "Selector", All: []string{"left", "right", "auto", "chain"}},
		"left":       {Name: "left", Type: "Selector", All: []string{"root", "shared"}},
		"right":      {Name: "right", Type: "Selector", All: []string{"shared"}},
		"shared":     {Name: "shared", Type: "Selector", All: []string{"n"}},
		"auto":       {Name: "auto", Type: "URLTest", All: []string{"n"}},
		"chain":      {Name: "chain", Type: "Relay", All: []string{"relay-only"}},
		"relay-only": {Name: "relay-only", Type: "Selector", All: []string{"n"}},
		"n":          {Name: "n", Type: "VLESS"},
	}
	options := m.nestedSelectors("root", proxies, nil, model.ScanRequest{})
	if len(options) != 1 || options[0].Group != "shared" || options[0].OnCurrentPath {
		t.Fatalf("shared/cyclic/relay resolution=%+v", options)
	}
	proxies["root"] = mihomo.Proxy{Name: "root", Type: "Selector", All: []string{"auto"}}
	if options := m.nestedSelectors("root", proxies, nil, model.ScanRequest{}); len(options) != 0 {
		t.Fatalf("automatic group offered as manually selectable: %+v", options)
	}
}

func TestPreflightDistinguishesEmptyFiltersFromNestedGroups(t *testing.T) {
	m := NewManager(config.Defaults(), &fakeMihomo{proxies: map[string]mihomo.Proxy{
		"work":  {Name: "work", Type: "Selector", All: []string{"JP-01"}},
		"JP-01": {Name: "JP-01", Type: "VLESS"},
	}}, nil)
	preview, err := m.Preflight(context.Background(), model.ScanRequest{TargetGroup: "work", ProfileID: "internet-baseline", Regions: []string{"US"}, Mode: "quick"})
	if err != nil || preview.Ready || preview.ReasonCode != "filters_empty" || len(preview.NestedSelectors) != 0 {
		t.Fatalf("filtered group misdiagnosed: %+v %v", preview, err)
	}
}

func TestNestedSelectorReportsSelectedPathWhenInactiveShortcutExists(t *testing.T) {
	m := NewManager(config.Defaults(), &fakeMihomo{}, nil)
	proxies := map[string]mihomo.Proxy{
		"root":   {Type: "Selector", Now: "middle", All: []string{"pool", "middle"}},
		"middle": {Type: "Selector", Now: "pool", All: []string{"pool"}},
		"pool":   {Type: "Selector", Now: "node", All: []string{"node"}},
		"node":   {Name: "node", Type: "VLESS"},
	}
	options := m.nestedSelectors("root", proxies, nil, model.ScanRequest{})
	if len(options) != 1 || !options[0].OnCurrentPath || !reflect.DeepEqual(options[0].Path, []string{"root", "middle", "pool"}) {
		t.Fatalf("selected shared path was hidden by a shorter inactive route: %+v", options)
	}
}
