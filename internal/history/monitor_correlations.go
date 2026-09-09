package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type correlationState struct {
	Signature  string
	Failures   int
	Recoveries int
	ActiveID   int64
	StartedAt  time.Time
}

func (s *Store) ObserveMonitorCorrelation(ctx context.Context, scope string, o model.MonitorCorrelation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	key := o.PlanID + ":" + o.Provider
	state := correlationState{}
	var data string
	err = tx.QueryRowContext(ctx, `SELECT payload FROM monitor_correlation_state WHERE scope=? AND key=?`, scope, key).Scan(&data)
	if err == nil {
		if err = json.Unmarshal([]byte(data), &state); err != nil {
			return err
		}
	} else if err != sql.ErrNoRows {
		return err
	}
	if state.Signature == o.Signature {
		return nil
	}
	state.Signature = o.Signature
	bad := o.Reliable && o.Comparable >= 3 && o.Failed*5 >= o.Comparable*4
	if bad {
		state.Failures++
		state.Recoveries = 0
	} else if o.Reliable && o.Comparable >= 3 {
		state.Recoveries++
		state.Failures = 0
	} else {
		state.Failures = 0
		state.Recoveries = 0
	}
	if state.ActiveID == 0 && state.Failures >= 2 {
		o.StartedAt = o.UpdatedAt
		o.Status = "active"
		payload, _ := json.Marshal(o)
		r, e := tx.ExecContext(ctx, `INSERT INTO monitor_correlations(scope,plan_id,provider,status,started_at,updated_at,payload) VALUES(?,?,?,?,?,?,?)`, scope, o.PlanID, o.Provider, o.Status, o.StartedAt.Unix(), o.UpdatedAt.Unix(), string(payload))
		if e != nil {
			return e
		}
		state.ActiveID, e = r.LastInsertId()
		if e != nil {
			return e
		}
		state.StartedAt = o.StartedAt
	}
	if state.ActiveID != 0 {
		o.ID = state.ActiveID
		o.StartedAt = state.StartedAt
		o.Status = "active"
		if !o.Reliable || o.Comparable < 3 {
			o.Status = "uncertain"
			o.Message = "观测不足或环境异常，暂不能确认此段异常是否恢复"
		}
		if state.Recoveries >= 2 {
			o.Status = "recovered"
			o.Message = "连续两轮新观测已不再满足共同异常条件"
		}
		payload, _ := json.Marshal(o)
		if _, err = tx.ExecContext(ctx, `UPDATE monitor_correlations SET status=?,updated_at=?,payload=? WHERE id=?`, o.Status, o.UpdatedAt.Unix(), string(payload), o.ID); err != nil {
			return err
		}
		if o.Status == "recovered" {
			state.ActiveID = 0
			state.Failures = 0
			state.Recoveries = 0
		}
	}
	payload, _ := json.Marshal(state)
	if _, err = tx.ExecContext(ctx, `INSERT INTO monitor_correlation_state(scope,key,plan_id,payload) VALUES(?,?,?,?) ON CONFLICT(scope,key) DO UPDATE SET payload=excluded.payload`, scope, key, o.PlanID, string(payload)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EndMonitorCorrelations(ctx context.Context, scope, plan string, paused bool, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE monitor_correlations SET status='scope_changed',updated_at=? WHERE scope=? AND status IN ('active','uncertain') AND (plan_id!=? OR ?)`, now.Unix(), scope, plan, paused); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM monitor_correlation_state WHERE scope=? AND (plan_id!=? OR ?)`, scope, plan, paused); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MonitorCorrelations(ctx context.Context, scope string) ([]model.MonitorCorrelation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,status,updated_at,payload FROM monitor_correlations WHERE scope=? ORDER BY CASE WHEN status IN ('active','uncertain') THEN 0 ELSE 1 END,updated_at DESC,id DESC LIMIT 100`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorCorrelation{}
	for rows.Next() {
		var id, updated int64
		var status, data string
		if err = rows.Scan(&id, &status, &updated, &data); err != nil {
			return nil, err
		}
		var o model.MonitorCorrelation
		if err = json.Unmarshal([]byte(data), &o); err != nil {
			return nil, err
		}
		o.ID = id
		o.Status = status
		o.UpdatedAt = time.Unix(updated, 0).UTC()
		if status == "scope_changed" {
			o.Message = "监控暂停或范围变化，结束此段观察；不代表网络恢复"
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
