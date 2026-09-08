package scan

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestAutomaticSwitchAuditCooldownMembershipAndUnresolved(t *testing.T) {
	for _, scenario := range []string{"confirmed", "disk", "changed-current", "identity", "unresolved", "cooldown", "strict", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.Defaults()
			path := filepath.Join(t.TempDir(), "db")
			store, err := history.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			f := &fakeMihomo{proxies: map[string]mihomo.Proxy{"g": {Name: "g", Type: "Selector", Now: "old", All: []string{"old", "new"}}, "old": {Name: "old", Type: "VLESS"}, "new": {Name: "new", Type: "VLESS"}}}
			m := NewManager(cfg, f, store)
			profile, nodes, _, err := m.MonitorCatalog(ctx, "g", "chatgpt")
			if err != nil {
				t.Fatal(err)
			}
			var target model.MonitorNode
			for _, n := range nodes {
				if n.Name == "new" {
					target = n
				}
			}
			p := model.MonitorPlan{ID: "p", Group: "g", ProfileID: "chatgpt", ProfileHash: MonitorProfileHash(profile), Enabled: true, AutoSwitch: true, Nodes: nodes}
			switch scenario {
			case "disk":
				raw, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer raw.Close()
				if _, err = raw.Exec(`CREATE TRIGGER fail_auto_audit BEFORE INSERT ON switch_events BEGIN SELECT RAISE(FAIL,'fixture full'); END`); err != nil {
					t.Fatal(err)
				}
			case "changed-current":
				g := f.proxies["g"]
				g.Now = "manual"
				f.proxies["g"] = g
			case "identity":
				n := f.proxies["new"]
				n.Type = "Trojan"
				f.proxies["new"] = n
			case "unresolved", "cooldown":
				status := "pending"
				if scenario == "cooldown" {
					status = "confirmed"
				}
				if _, err = store.RecordSwitch(ctx, model.SwitchEvent{Group: "g", Status: status, CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
			case "strict":
				for i := range cfg.Scanner.ProbeProfiles {
					if cfg.Scanner.ProbeProfiles[i].ID == "chatgpt" {
						cfg.Scanner.ProbeProfiles[i].RequireStrict = true
					}
				}
				m.cfg.Store(&cfg)
			case "disabled":
				p.AutoSwitch = false
			}
			event, err := m.MonitorSwitch(ctx, p, "old", target, "automatic-request")
			if scenario != "confirmed" {
				if err == nil || f.selected != "" {
					t.Fatalf("unsafe automatic switch: %+v %v", event, err)
				}
				return
			}
			if err != nil || event.Status != "confirmed" || !event.AuditPersisted || event.ScanID != "monitor:p" || f.selected != "new" {
				t.Fatalf("switch failed: %+v %v", event, err)
			}
			again, err := m.MonitorSwitch(ctx, p, "old", target, "automatic-request")
			if err != nil || again.ID != event.ID {
				t.Fatalf("idempotent retry: %+v %v", again, err)
			}
			if _, err = m.MonitorSwitch(ctx, p, "new", nodes[0], "second"); err == nil {
				t.Fatal("cooldown not persistent")
			}
		})
	}
}

func TestAutomaticSwitchUnknownReadbackBlocksAnotherAttempt(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	store, err := history.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	f := &uncertainController{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{"g": {Name: "g", Type: "Selector", Now: "old", All: []string{"old", "new"}}, "old": {Name: "old", Type: "VLESS"}, "new": {Name: "new", Type: "VLESS"}}}}
	m := NewManager(cfg, f, store)
	profile, nodes, _, err := m.MonitorCatalog(ctx, "g", "chatgpt")
	if err != nil {
		t.Fatal(err)
	}
	var target model.MonitorNode
	for _, n := range nodes {
		if n.Name == "new" {
			target = n
		}
	}
	p := model.MonitorPlan{ID: "p", Enabled: true, AutoSwitch: true, Group: "g", ProfileID: "chatgpt", ProfileHash: MonitorProfileHash(profile), Nodes: nodes}
	f.afterPut = func() { f.readFails = true }
	event, err := m.MonitorSwitch(ctx, p, "old", target, "first")
	if err != nil || event.Status != "unknown" || !event.AuditPersisted {
		t.Fatal(event, err)
	}
	f.readFails = false
	if _, err = m.MonitorSwitch(ctx, p, "old", target, "second"); err == nil || f.calls != 1 {
		t.Fatal("unknown outcome replayed", err, f.calls)
	}
	if _, err = m.ReconcileSwitch(ctx, event.ID); err != nil {
		t.Fatal(err)
	}
}
