package history

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type activityCursor struct {
	At     int64
	Source string
	ID     int64
}

func (s *Store) MonitorActivities(ctx context.Context, scope string, from, to time.Time, cursor string, limit int) (model.MonitorActivityPage, error) {
	out := model.MonitorActivityPage{Items: []model.MonitorActivity{}}
	if limit < 1 || limit > 200 || !to.After(from) || to.Sub(from) > 90*24*time.Hour {
		return out, fmt.Errorf("无效事件范围或分页大小")
	}
	var after *activityCursor
	if cursor != "" {
		if len(cursor) > 256 {
			return out, fmt.Errorf("无效游标")
		}
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return out, fmt.Errorf("无效游标")
		}
		var c activityCursor
		if json.Unmarshal(data, &c) != nil || c.ID < 0 {
			return out, fmt.Errorf("无效游标")
		}
		if c.Source != "node" && c.Source != "plan" && c.Source != "switch" && c.Source != "provider" && c.Source != "environment" {
			return out, fmt.Errorf("无效游标")
		}
		after = &c
	}
	type source struct{ name, query string }
	sources := []source{
		{"node", `SELECT e.id,e.at,json_set(e.payload,'$.series_id',COALESCE((SELECT b.series_id FROM monitor_bindings b WHERE b.plan_id=e.plan_id AND b.node_id=json_extract(e.payload,'$.node_id')),''),'$.group',COALESCE((SELECT json_extract(r.payload,'$.group') FROM monitor_revisions r WHERE r.plan_id=e.plan_id ORDER BY r.revision DESC LIMIT 1),'')) AS payload FROM monitor_events e WHERE e.plan_id IN (SELECT plan_id FROM monitor_revisions WHERE scope=?)`},
		{"environment", `SELECT id,at,json_object('status',status,'message',message) AS payload FROM monitor_system_events WHERE scope=?`},
		{"plan", `SELECT revision AS id,at,payload FROM monitor_revisions WHERE scope=?`},
		{"switch", `SELECT id,CAST(strftime('%s',created_at) AS INTEGER) AS at,json_object('status',status,'group',group_name,'previous',previous_member,'selected',selected_member,'scan_id',scan_id,'message',reason) AS payload FROM switch_events WHERE group_name IN (SELECT json_extract(payload,'$.group') FROM monitor_revisions WHERE scope=?)`},
		{"provider", `SELECT id,updated_at AS at,json_set(payload,'$.status',status) AS payload FROM monitor_correlations WHERE scope=?`},
	}
	for _, source := range sources {
		query := `SELECT id,at,payload FROM (` + source.query + `) WHERE at>=? AND at<=?`
		args := []any{scope, from.Unix(), to.Unix()}
		if after != nil {
			if source.name < after.Source {
				query += ` AND at<=?`
				args = append(args, after.At)
			} else if source.name > after.Source {
				query += ` AND at<?`
				args = append(args, after.At)
			} else {
				query += ` AND (at<? OR (at=? AND id<?))`
				args = append(args, after.At, after.At, after.ID)
			}
		}
		query += ` ORDER BY at DESC,id DESC LIMIT ?`
		args = append(args, limit+1)
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var id, at int64
			var data string
			if err = rows.Scan(&id, &at, &data); err != nil {
				rows.Close()
				return out, err
			}
			var v struct {
				Status               string          `json:"status"`
				Node                 string          `json:"node_name"`
				Group                string          `json:"group"`
				SeriesID             string          `json:"series_id"`
				Message              string          `json:"message"`
				Previous             string          `json:"previous"`
				Selected             string          `json:"selected"`
				ScanID               string          `json:"scan_id"`
				Provider             string          `json:"provider"`
				Revision             int             `json:"revision"`
				Enabled              bool            `json:"enabled"`
				AutoSwitch           bool            `json:"auto_switch"`
				Nodes                json.RawMessage `json:"nodes"`
				Failed               int             `json:"failed"`
				Comparable           int             `json:"comparable"`
				OtherProviderHealthy bool            `json:"other_provider_healthy"`
			}
			if err = json.Unmarshal([]byte(data), &v); err != nil {
				rows.Close()
				return out, err
			}
			a := model.MonitorActivity{Key: fmt.Sprintf("%s:%020d", source.name, id), At: time.Unix(at, 0).UTC(), Kind: source.name, Status: v.Status, Node: v.Node, SeriesID: v.SeriesID, Group: v.Group, Message: v.Message, Previous: v.Previous, Selected: v.Selected}
			switch source.name {
			case "switch":
				a.Kind = "manual_switch"
				if strings.HasPrefix(v.ScanID, "monitor:") {
					a.Kind = "automatic_switch"
				}
				a.Message = v.Previous + " → " + v.Selected + "；" + v.Message
			case "plan":
				var members []model.MonitorNode
				if err = json.Unmarshal(v.Nodes, &members); err != nil {
					rows.Close()
					return out, err
				}
				a.Status = "running"
				if !v.Enabled {
					a.Status = "paused"
				}
				a.Message = fmt.Sprintf("方案修订 %d：%d 个节点，监控运行=%t，自动切换=%t", v.Revision, len(members), v.Enabled, v.AutoSwitch)
			case "provider":
				a.Node = v.Provider
				if len(v.Nodes) > 0 {
					if err = json.Unmarshal(v.Nodes, &a.RelatedNodes); err != nil {
						rows.Close()
						return out, err
					}
				}
				a.Failed = v.Failed
				a.Comparable = v.Comparable
				a.OtherProviderHealthy = v.OtherProviderHealthy
				if a.Status == "scope_changed" {
					a.Message = "监控范围变化，结束此段观察，不代表网络恢复"
				}
			}
			out.Items = append(out.Items, a)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		return a.Key > b.Key
	})
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[len(out.Items)-1]
		parts := strings.SplitN(last.Key, ":", 2)
		id, _ := strconv.ParseInt(parts[1], 10, 64)
		data, _ := json.Marshal(activityCursor{At: last.At.Unix(), Source: parts[0], ID: id})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return out, nil
}
