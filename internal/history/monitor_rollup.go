package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Per-hour compact slots retain exact successful delays and failure boundaries.
// [baseline slot, outcome, delay ms, completed Unix seconds, reason code].
type hourPayload struct {
	Slots [][5]int64 `json:"slots"`
}

var reasonCodes = []string{"legacy_unclassified", "ok", "node_path", "controller", "budget", "busy", "node_missing", "unknown", "environment"}

func reasonIndex(s string) int64 {
	for i, v := range reasonCodes {
		if v == s {
			return int64(i)
		}
	}
	return 7
}
func outcomeIndex(s string) int64 {
	switch s {
	case "success":
		return 1
	case "failure":
		return 2
	default:
		return 0
	}
}
func outcomeName(i int64) string {
	switch i {
	case 1:
		return "success"
	case 2:
		return "failure"
	default:
		return "unknown"
	}
}

func (s *Store) planSeries(ctx context.Context, plan string) ([]model.MonitorSeries, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.payload FROM monitor_series t JOIN monitor_bindings b ON b.series_id=t.id WHERE b.plan_id=? ORDER BY t.anchor,t.id`, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorSeries{}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var v model.MonitorSeries
		if err = json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SeriesSamples(ctx context.Context, series model.MonitorSeries, from, to time.Time, extras bool) ([]model.MonitorSample, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("无效时间范围")
	}
	estimate := max(int64(0), (to.Unix()-from.Unix())/120+2)
	if estimate > 6000 {
		estimate = 1000
	}
	bySlot := make(map[int64]model.MonitorSample, int(estimate))
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM monitor_hourly WHERE series_id=? AND hour>=? AND hour<=? ORDER BY hour`, series.ID, from.Unix()/3600*3600, to.Unix()/3600*3600)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return nil, err
		}
		var hour hourPayload
		if err = json.Unmarshal([]byte(data), &hour); err != nil {
			rows.Close()
			return nil, err
		}
		for _, slot := range hour.Slots {
			scheduled := series.Anchor + slot[0]*120
			if scheduled < from.Unix() || scheduled > to.Unix() {
				continue
			}
			code := "unknown"
			if slot[4] >= 0 && slot[4] < int64(len(reasonCodes)) {
				code = reasonCodes[slot[4]]
			}
			bySlot[slot[0]] = model.MonitorSample{NodeID: series.Node.ID, SeriesID: series.ID, Kind: "baseline", Slot: slot[0], ScheduledAt: scheduled, At: time.Unix(slot[3], 0).UTC(), Outcome: outcomeName(slot[1]), DelayMS: int(slot[2]), ReasonCode: code, DataVersion: 2, Resolution: "hourly"}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	query := `SELECT payload FROM monitor_observations WHERE series_id=? AND scheduled_at>=? AND scheduled_at<=?`
	if !extras {
		query += ` AND kind='baseline'`
	}
	query += ` ORDER BY scheduled_at,kind,slot`
	rows, err = s.db.QueryContext(ctx, query, series.ID, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorSample{}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var v model.MonitorSample
		if err = json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		v.Resolution = "raw"
		if v.Kind == "baseline" {
			bySlot[v.Slot] = v
		} else {
			out = append(out, v)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	merged := make([]model.MonitorSample, 0, len(out)+len(bySlot))
	merged = append(merged, out...)
	for _, v := range bySlot {
		merged = append(merged, v)
	}
	out = merged
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ScheduledAt != out[j].ScheduledAt {
			return out[i].ScheduledAt < out[j].ScheduledAt
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// Rebuild a bounded number of dirty hours. Read + replace + dirty acknowledgement
// share a transaction so a concurrent late sample cannot be lost between them.
func (s *Store) AggregateMonitor(ctx context.Context, now time.Time) error {
	_, err := s.AggregateMonitorBatch(ctx, now, 24)
	return err
}

func (s *Store) AggregateMonitorBatch(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit < 1 || limit > 24 {
		return 0, fmt.Errorf("aggregation batch must be between 1 and 24 hours")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT series_id,hour FROM monitor_dirty_hours WHERE hour<=? ORDER BY hour,series_id LIMIT ?`, now.Add(-2*time.Minute).Unix()-3600, limit)
	if err != nil {
		return 0, err
	}
	type key struct {
		id   string
		hour int64
	}
	keys := []key{}
	for rows.Next() {
		var v key
		if err = rows.Scan(&v.id, &v.hour); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, k := range keys {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		r, e := tx.QueryContext(ctx, `SELECT payload FROM monitor_observations WHERE series_id=? AND kind='baseline' AND scheduled_at>=? AND scheduled_at<? ORDER BY slot`, k.id, k.hour, k.hour+3600)
		if e != nil {
			tx.Rollback()
			return 0, e
		}
		// Merge existing compact slots when raw retention has already removed
		// earlier entries and a legitimate late sample dirties this hour again.
		var old string
		slots := map[int64][5]int64{}
		items := []model.MonitorSample{}
		for r.Next() {
			var data string
			if e = r.Scan(&data); e != nil {
				break
			}
			var v model.MonitorSample
			if e = json.Unmarshal([]byte(data), &v); e != nil {
				break
			}
			items = append(items, v)
		}
		if e == nil {
			e = r.Err()
		}
		r.Close()
		if e != nil {
			tx.Rollback()
			return 0, e
		}
		e = tx.QueryRowContext(ctx, `SELECT payload FROM monitor_hourly WHERE series_id=? AND hour=?`, k.id, k.hour).Scan(&old)
		if e == nil {
			var h hourPayload
			if e = json.Unmarshal([]byte(old), &h); e != nil {
				tx.Rollback()
				return 0, e
			}
			for _, v := range h.Slots {
				slots[v[0]] = v
			}
		} else if e != sql.ErrNoRows {
			tx.Rollback()
			return 0, e
		}
		for _, v := range items {
			slots[v.Slot] = [5]int64{v.Slot, outcomeIndex(v.Outcome), int64(v.DelayMS), v.At.Unix(), reasonIndex(v.ReasonCode)}
		}
		h := hourPayload{Slots: [][5]int64{}}
		for _, v := range slots {
			h.Slots = append(h.Slots, v)
		}
		sort.Slice(h.Slots, func(i, j int) bool { return h.Slots[i][0] < h.Slots[j][0] })
		data, e := json.Marshal(h)
		if e == nil {
			_, e = tx.ExecContext(ctx, `INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) VALUES(?,?,?,?) ON CONFLICT(series_id,hour) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at`, k.id, k.hour, string(data), now.Unix())
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, `DELETE FROM monitor_dirty_hours WHERE series_id=? AND hour=?`, k.id, k.hour)
		}
		if e != nil {
			tx.Rollback()
			return 0, e
		}
		if e = tx.Commit(); e != nil {
			return 0, e
		}
	}
	return len(keys), nil
}
