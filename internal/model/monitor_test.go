package model

import (
	"encoding/json"
	"testing"
)

func TestMonitorPlanToleranceJSONCompatibility(t *testing.T) {
	for _, tt := range []struct {
		payload string
		want    float64
	}{
		{`{"id":"legacy","auto_switch":true}`, DefaultFailoverTolerancePP},
		{`{"id":"zero","failover_tolerance_pp":0}`, 0},
		{`{"id":"custom","failover_tolerance_pp":2.5}`, 2.5},
	} {
		var plan MonitorPlan
		if err := json.Unmarshal([]byte(tt.payload), &plan); err != nil {
			t.Fatal(err)
		}
		if plan.FailoverTolerancePP != tt.want {
			t.Fatalf("%s: got %g, want %g", tt.payload, plan.FailoverTolerancePP, tt.want)
		}
		data, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip MonitorPlan
		if err = json.Unmarshal(data, &roundtrip); err != nil || roundtrip.FailoverTolerancePP != tt.want {
			t.Fatal(string(data), err)
		}
	}
}
