package history

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Read cache state before committing a plan revision. Corrupt stored state must
// not advance the durable revision while leaving the running manager behind it.
func (s *Store) MonitorCheckpoint(ctx context.Context, nodes []model.MonitorNode) (map[string]model.MonitorState, map[string]int64, error) {
	states := map[string]model.MonitorState{}
	slots := map[string]int64{}
	for _, n := range nodes {
		var data string
		err := s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_series_states WHERE series_id=?`, n.SeriesID).Scan(&data)
		if err == nil {
			var state model.MonitorState
			if err = json.Unmarshal([]byte(data), &state); err != nil {
				return nil, nil, err
			}
			states[n.ID] = state
		} else if err != sql.ErrNoRows {
			return nil, nil, err
		}
		rows, err := s.db.QueryContext(ctx, `SELECT kind,MAX(slot) FROM monitor_observations WHERE series_id=? GROUP BY kind`, n.SeriesID)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var kind string
			var slot int64
			if err = rows.Scan(&kind, &slot); err != nil {
				rows.Close()
				return nil, nil, err
			}
			slots[n.ID+"/"+kind] = slot
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
		err = s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_hourly WHERE series_id=? ORDER BY hour DESC LIMIT 1`, n.SeriesID).Scan(&data)
		if err == nil {
			var hour hourPayload
			if err = json.Unmarshal([]byte(data), &hour); err != nil {
				return nil, nil, err
			}
			key := n.ID + "/baseline"
			for _, v := range hour.Slots {
				if last, ok := slots[key]; !ok || v[0] > last {
					slots[key] = v[0]
				}
			}
		} else if err != sql.ErrNoRows {
			return nil, nil, err
		}
	}
	return states, slots, nil
}
