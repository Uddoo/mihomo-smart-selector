package history

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// A metric needs a slot, outcome and exact delay, not a complete sample object.
// Timestamp/reason metadata is retained only to reconstruct the final 60 samples.
type compactBaseline struct {
	slot, at        int64
	delay           int
	outcome, reason uint8
	raw             bool
}

func compactOutcome(value string) uint8 {
	switch value {
	case "unknown":
		return 1
	case "success":
		return 2
	default:
		return 3
	}
}
func baselineOutcome(value uint8) string {
	switch value {
	case 1:
		return "unknown"
	case 2:
		return "success"
	default:
		return "failure"
	}
}

// Keep at most the 60 largest raw slots. This also handles out-of-order stored
// schedules without retaining every raw JSON payload or relying on query order.
type baselineTail struct {
	slots   []int64
	payload map[int64]string
}

func (t *baselineTail) keep(slot int64, data string) {
	if t.payload == nil {
		t.payload = make(map[int64]string, 60)
	}
	if _, exists := t.payload[slot]; exists {
		t.payload[slot] = data
		return
	}
	if len(t.slots) == 60 {
		if slot < t.slots[0] {
			return
		}
		delete(t.payload, t.slots[0])
		t.slots[0] = slot
		for i := 0; ; {
			child := 2*i + 1
			if child >= len(t.slots) {
				break
			}
			if child+1 < len(t.slots) && t.slots[child+1] < t.slots[child] {
				child++
			}
			if t.slots[i] <= t.slots[child] {
				break
			}
			t.slots[i], t.slots[child] = t.slots[child], t.slots[i]
			i = child
		}
	} else {
		t.slots = append(t.slots, slot)
		for i := len(t.slots) - 1; i > 0; {
			parent := (i - 1) / 2
			if t.slots[parent] <= t.slots[i] {
				break
			}
			t.slots[parent], t.slots[i] = t.slots[i], t.slots[parent]
			i = parent
		}
	}
	t.payload[slot] = data
}

// VisitSeriesBaseline is the bounded overview path. It preserves the same raw
// override and inclusive time bounds as SeriesSamples. Full history/diagnostic
// callers retain that API; only metrics and an owned recent tail leave this one.
func (s *Store) VisitSeriesBaseline(ctx context.Context, series model.MonitorSeries, from, to time.Time, visit func(int64, string, int)) ([]model.MonitorSample, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("无效时间范围")
	}
	first := max(int64(0), (from.Unix()-series.Anchor+119)/120)
	last := (to.Unix() - series.Anchor) / 120
	var dense []compactBaseline
	if size := last - first + 1; size > 0 && size <= 6000 {
		dense = make([]compactBaseline, int(size))
	}
	var overflow map[int64]compactBaseline
	count := 0
	put := func(point compactBaseline) {
		index := point.slot - first
		if index >= 0 && index < int64(len(dense)) {
			if dense[index].outcome == 0 {
				count++
			}
			dense[index] = point
		} else {
			if overflow == nil {
				overflow = map[int64]compactBaseline{}
			}
			if _, found := overflow[point.slot]; !found {
				count++
			}
			overflow[point.slot] = point
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM monitor_hourly WHERE series_id=? AND hour>=? AND hour<=? ORDER BY hour`, series.ID, from.Unix()/3600*3600, to.Unix()/3600*3600)
	if err != nil {
		return nil, err
	}
	hour := hourPayload{Slots: make([][5]int64, 0, 30)}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return nil, err
		}
		hour.Slots = hour.Slots[:0]
		if err = json.Unmarshal([]byte(data), &hour); err != nil {
			rows.Close()
			return nil, err
		}
		for _, slot := range hour.Slots {
			scheduled := series.Anchor + slot[0]*120
			if scheduled < from.Unix() || scheduled > to.Unix() {
				continue
			}
			reason := uint8(7)
			if slot[4] >= 0 && slot[4] < int64(len(reasonCodes)) {
				reason = uint8(slot[4])
			}
			put(compactBaseline{slot: slot[0], at: slot[3], delay: int(slot[2]), outcome: compactOutcome(outcomeName(slot[1])), reason: reason})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT payload FROM monitor_observations WHERE series_id=? AND scheduled_at>=? AND scheduled_at<=? AND kind='baseline' ORDER BY scheduled_at,kind,slot`, series.ID, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	var raw struct {
		Slot    int64  `json:"slot"`
		Kind    string `json:"kind"`
		Outcome string `json:"outcome"`
		Delay   int    `json:"delay_ms"`
	}
	tail := baselineTail{}
	fallback := false
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return nil, err
		}
		raw.Slot, raw.Kind, raw.Outcome, raw.Delay = 0, "", "", 0
		if err = json.Unmarshal([]byte(data), &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if raw.Kind != "baseline" {
			fallback = true
			continue
		}
		put(compactBaseline{slot: raw.Slot, delay: raw.Delay, outcome: compactOutcome(raw.Outcome), raw: true})
		tail.keep(raw.Slot, data)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if fallback {
		// Preserve legacy payloads whose JSON kind disagrees with their SQL key.
		samples, err := s.SeriesSamples(ctx, series, from, to, false)
		if err != nil {
			return nil, err
		}
		for _, sample := range samples {
			if sample.Kind == "baseline" {
				visit(sample.Slot, sample.Outcome, sample.DelayMS)
			}
		}
		return append([]model.MonitorSample{}, samples[max(0, len(samples)-60):]...), nil
	}
	out := make([]model.MonitorSample, 0, min(60, count))
	consume := func(point compactBaseline) error {
		if point.outcome == 0 {
			return nil
		}
		visit(point.slot, baselineOutcome(point.outcome), point.delay)
		count--
		if count >= 60 {
			return nil
		}
		var sample model.MonitorSample
		if point.raw {
			data, found := tail.payload[point.slot]
			if !found {
				return fmt.Errorf("recent baseline payload unavailable")
			}
			if err := json.Unmarshal([]byte(data), &sample); err != nil {
				return err
			}
			sample.Resolution = "raw"
		} else {
			sample = model.MonitorSample{NodeID: series.Node.ID, SeriesID: series.ID, Kind: "baseline", Slot: point.slot, ScheduledAt: series.Anchor + point.slot*120, At: time.Unix(point.at, 0).UTC(), Outcome: baselineOutcome(point.outcome), DelayMS: point.delay, ReasonCode: reasonCodes[point.reason], DataVersion: 2, Resolution: "hourly"}
		}
		out = append(out, sample)
		return nil
	}
	if len(overflow) == 0 {
		for _, point := range dense {
			if err := consume(point); err != nil {
				return nil, err
			}
		}
	} else {
		keys := make([]int64, 0, count)
		for _, point := range dense {
			if point.outcome != 0 {
				keys = append(keys, point.slot)
			}
		}
		for slot := range overflow {
			keys = append(keys, slot)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, slot := range keys {
			point, ok := overflow[slot]
			if !ok {
				point = dense[slot-first]
			}
			if err := consume(point); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
