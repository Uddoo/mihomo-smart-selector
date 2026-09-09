package history

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type DiagnosticRecords struct {
	Samples   []model.MonitorSample
	Series    map[string]model.MonitorSeries
	Truncated bool
}

func (s *Store) DiagnosticSamples(ctx context.Context, scope string, from, to time.Time, limit int, legacy bool) (DiagnosticRecords, error) {
	out := DiagnosticRecords{Samples: []model.MonitorSample{}, Series: map[string]model.MonitorSeries{}}
	seen := map[string]bool{}
	add := func(v model.MonitorSample, series model.MonitorSeries) bool {
		key := fmt.Sprintf("%s/%s/%d", v.SeriesID, v.Kind, v.Slot)
		if seen[key] {
			return true
		}
		if len(out.Samples) >= limit {
			out.Truncated = true
			return false
		}
		seen[key] = true
		out.Samples = append(out.Samples, v)
		out.Series[series.ID] = series
		return true
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.payload,t.payload FROM monitor_observations r JOIN monitor_series t ON t.id=r.series_id WHERE t.scope=? AND r.scheduled_at>=? AND r.scheduled_at<=? ORDER BY r.scheduled_at DESC,r.series_id,r.kind LIMIT ?`, scope, from.Unix(), to.Unix(), limit+1)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var a, b string
		if err = rows.Scan(&a, &b); err != nil {
			rows.Close()
			return out, err
		}
		var v model.MonitorSample
		var series model.MonitorSeries
		if err = json.Unmarshal([]byte(a), &v); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal([]byte(b), &series); err != nil {
			rows.Close()
			return out, err
		}
		v.Resolution = "raw"
		if !add(v, series) {
			break
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if !out.Truncated {
		rows, err = s.db.QueryContext(ctx, `SELECT h.payload,t.payload FROM monitor_hourly h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=? AND h.hour>=? AND h.hour<=? ORDER BY h.hour DESC,h.series_id LIMIT ?`, scope, from.Unix()/3600*3600, to.Unix()/3600*3600, limit+1)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var a, b string
			if err = rows.Scan(&a, &b); err != nil {
				rows.Close()
				return out, err
			}
			var h hourPayload
			var series model.MonitorSeries
			if err = json.Unmarshal([]byte(a), &h); err != nil {
				rows.Close()
				return out, err
			}
			if err = json.Unmarshal([]byte(b), &series); err != nil {
				rows.Close()
				return out, err
			}
			for _, slot := range h.Slots {
				scheduled := series.Anchor + slot[0]*120
				if scheduled < from.Unix() || scheduled > to.Unix() {
					continue
				}
				code := "unknown"
				if slot[4] >= 0 && slot[4] < int64(len(reasonCodes)) {
					code = reasonCodes[slot[4]]
				}
				v := model.MonitorSample{SeriesID: series.ID, NodeID: series.Node.ID, Kind: "baseline", Slot: slot[0], ScheduledAt: scheduled, At: time.Unix(slot[3], 0).UTC(), Outcome: outcomeName(slot[1]), DelayMS: int(slot[2]), ReasonCode: code, Resolution: "hourly", DataVersion: 2}
				if !add(v, series) {
					break
				}
			}
			if out.Truncated {
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	if legacy && !out.Truncated {
		rows, err = s.db.QueryContext(ctx, `SELECT r.plan_id,r.payload FROM monitor_samples r WHERE r.at>=? AND r.at<=? AND NOT EXISTS(SELECT 1 FROM monitor_bindings b WHERE b.plan_id=r.plan_id AND b.node_id=r.node_id) ORDER BY r.at DESC LIMIT ?`, from.Unix(), to.Unix(), limit-len(out.Samples)+1)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var plan, data string
			if err = rows.Scan(&plan, &data); err != nil {
				rows.Close()
				return out, err
			}
			var v model.MonitorSample
			if err = json.Unmarshal([]byte(data), &v); err != nil {
				rows.Close()
				return out, err
			}
			v.SeriesID = "legacy:" + plan + ":" + v.NodeID
			v.Reason = ""
			v.ReasonCode = "legacy_unclassified"
			v.Resolution = "unmapped_legacy"
			v.ScheduledAt = 0
			series := model.MonitorSeries{ID: v.SeriesID, Node: model.MonitorNode{ID: v.NodeID, Name: "unmapped legacy node"}, ProfileID: "unknown"}
			if !add(v, series) {
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	sort.SliceStable(out.Samples, func(i, j int) bool { return out.Samples[i].At.Before(out.Samples[j].At) })
	return out, nil
}
