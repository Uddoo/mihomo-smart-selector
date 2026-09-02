package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestStoreRoundTripsScanAndSwitch(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "data", "selector.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	started := time.Now().UTC().Round(0)
	scan := model.Scan{
		ID: "scan-1", Status: model.ScanRunning, StartedAt: started,
		Request: model.ScanRequest{TargetGroup: "ChatGPT", Regions: []string{"JP"}, Mode: "stable"},
		Profile: model.ProbeProfileSummary{ID: "chatgpt", Label: "ChatGPT service", ProbeCount: 1, TransportScope: "HTTP latency only"},
	}
	if err := store.CreateScan(context.Background(), scan); err != nil {
		t.Fatal(err)
	}
	completed := started.Add(time.Second)
	scan.Status = model.ScanComplete
	scan.CompletedAt = &completed
	scan.Results = []model.NodeResult{{Rank: 1, Name: "JP-03", Score: 94.5, Samples: []model.ProbeSample{{Probe: "trace", DelayMS: 123}}}}
	if err := store.CompleteScan(context.Background(), scan); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetScan(context.Background(), scan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.ScanComplete || len(got.Results) != 1 || got.Results[0].Name != "JP-03" || got.Profile.ID != "chatgpt" {
		t.Fatalf("round trip scan = %#v", got)
	}
	event, err := store.RecordSwitch(context.Background(), model.SwitchEvent{
		ScanID: scan.ID, Group: "ChatGPT", Previous: "JP-01", Selected: "JP-03", Reason: "manual", CreatedAt: completed,
	})
	if err != nil || event.ID == 0 {
		t.Fatalf("record switch = %#v, %v", event, err)
	}
	events, err := store.ListSwitches(context.Background(), 10)
	if err != nil || len(events) != 1 || events[0].Selected != "JP-03" {
		t.Fatalf("switches = %#v, %v", events, err)
	}
}
