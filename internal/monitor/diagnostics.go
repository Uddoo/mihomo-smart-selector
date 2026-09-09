package monitor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) Activities(ctx context.Context, from, to time.Time, cursor string, limit int) (model.MonitorActivityPage, error) {
	return m.store.MonitorActivities(ctx, m.source.MonitorScope(), from, to, cursor, limit)
}

type DiagnosticRequest struct {
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	IncludeNames  bool      `json:"include_names"`
	IncludeLegacy bool      `json:"include_legacy"`
}

func safeCSV(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + s
	}
	return s
}

func (m *Manager) Diagnostics(ctx context.Context, r DiagnosticRequest) ([]byte, error) {
	release, budgetErr := m.acquireHistory(ctx)
	if budgetErr != nil {
		return nil, budgetErr
	}
	defer release()
	now := m.now()
	if r.To.IsZero() {
		r.To = now
	}
	if r.From.IsZero() {
		r.From = r.To.Add(-time.Hour)
	}
	if !r.To.After(r.From) || r.To.After(now.Add(time.Minute)) || r.To.Sub(r.From) > 7*24*time.Hour {
		return nil, fmt.Errorf("诊断导出范围最多7天，结束时间不能在未来")
	}
	records, err := m.store.DiagnosticSamples(ctx, m.source.MonitorScope(), r.From, r.To, 20000, r.IncludeLegacy)
	if err != nil {
		return nil, err
	}
	activities := []model.MonitorActivity{}
	cursor := ""
	eventTruncated := false
	for len(activities) < 2000 {
		page, e := m.Activities(ctx, r.From, r.To, cursor, 200)
		if e != nil {
			return nil, e
		}
		activities = append(activities, page.Items...)
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
		if len(activities) >= 2000 {
			eventTruncated = true
		}
	}
	aliases := map[string]string{}
	sequence := map[string]int{}
	alias := func(kind, value string) string {
		if value == "" {
			return ""
		}
		key := kind + ":" + value
		if v, ok := aliases[key]; ok {
			return v
		}
		sequence[kind]++
		v := fmt.Sprintf("%s-%d", kind, sequence[kind])
		aliases[key] = v
		return v
	}
	name := func(kind, value string) string {
		if r.IncludeNames {
			return value
		}
		return alias(kind, value)
	}
	ids := []string{}
	for id := range records.Series {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	definitions := []map[string]any{}
	for _, id := range ids {
		s := records.Series[id]
		definitions = append(definitions, map[string]any{"series": alias("series", id), "node": name("node", s.Node.Name), "provider": name("provider", s.Node.Provider), "profile": name("profile", s.ProfileID), "first_observed_unix": s.Anchor, "definition_known": !strings.HasPrefix(id, "legacy:")})
	}
	var csvBytes bytes.Buffer
	writer := csv.NewWriter(&csvBytes)
	writer.Write([]string{"series", "node", "provider", "profile", "scheduled_at_utc", "completed_at_utc", "kind", "outcome", "delay_ms", "reason_code", "resolution", "data_version"})
	baseline, failures, unknown := 0, 0, 0
	for _, v := range records.Samples {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		s := records.Series[v.SeriesID]
		scheduled := ""
		if v.ScheduledAt != 0 {
			scheduled = time.Unix(v.ScheduledAt, 0).UTC().Format(time.RFC3339)
		}
		row := []string{alias("series", v.SeriesID), name("node", s.Node.Name), name("provider", s.Node.Provider), name("profile", s.ProfileID), scheduled, v.At.UTC().Format(time.RFC3339), v.Kind, v.Outcome, strconv.Itoa(v.DelayMS), v.ReasonCode, v.Resolution, strconv.Itoa(v.DataVersion)}
		for i := range row {
			row[i] = safeCSV(row[i])
		}
		if err = writer.Write(row); err != nil {
			return nil, err
		}
		if v.Kind == "baseline" {
			baseline++
			if v.Outcome == "failure" {
				failures++
			}
			if v.Outcome == "unknown" {
				unknown++
			}
		}
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		return nil, err
	}
	switches := []model.MonitorActivity{}
	incidents := []model.MonitorActivity{}
	for _, a := range activities {
		a.Key = alias("event", a.Key)
		a.SeriesID = alias("series", a.SeriesID)
		a.Group = name("group", a.Group)
		if a.Kind == "provider" {
			a.Node = name("provider", a.Node)
			for i := range a.RelatedNodes {
				a.RelatedNodes[i] = name("node", a.RelatedNodes[i])
			}
		} else {
			a.Node = name("node", a.Node)
		}
		if a.Kind == "automatic_switch" || a.Kind == "manual_switch" {
			a.Previous = name("node", a.Previous)
			a.Selected = name("node", a.Selected)
			a.Message = a.Previous + " → " + a.Selected + "；" + a.Status
			switches = append(switches, a)
		} else {
			a.Message = "事件类型：" + a.Kind + "；状态：" + a.Status
			if a.Kind == "provider" {
				a.Message += fmt.Sprintf("；异常 %d/%d，有其他 Provider 成功对照=%t", a.Failed, a.Comparable, a.OtherProviderHealthy)
			}
			incidents = append(incidents, a)
		}
	}
	manifest := map[string]any{"generated_at": now, "from": r.From, "to": r.To, "timezone": "UTC", "schema_version": 2, "score_version": "https-70-20-10-v1", "auto_switch_window": "24h", "include_names": r.IncludeNames, "include_legacy": r.IncludeLegacy, "sample_count": len(records.Samples), "sample_truncated": records.Truncated, "event_truncated": eventTruncated, "series": definitions, "redactions": []string{"credentials", "controller address", "subscription and probe URLs", "raw error messages", "chat content", "connection lists"}, "notes": []string{"基准与附加检查分开；时隙字段用于指标，完成时间用于请求事件", "聚合记录保留基准时隙与延迟，详细原始原因可能已过期", "未映射旧记录没有可靠方案定义，不参与正式排名", "此包中的失败计数只涵盖导出记录，不是完整窗口可用率；截断和缺测不能视为成功"}}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	incidentData, _ := json.MarshalIndent(incidents, "", "  ")
	switchData, _ := json.MarshalIndent(switches, "", "  ")
	summary := fmt.Sprintf("# 监控诊断摘要\n\n时间范围（UTC）：%s 至 %s\n\n导出 %d 条记录，其中基准 %d 条、失败 %d 条、未知 %d 条。\n\n样本截断：%t；事件截断：%t。这些计数不能替代包含覆盖率的完整窗口评分。\n\n切换记录 %d 条；其余事件 %d 条。具体情况见 CSV 和 JSON。\n\n只记录已观察到的失败和切换，不据此断言节点服务器或运营商是根因。长连接未验证。默认名称使用包内一致别名，凭据、地址、原始错误文本及聊天内容不导出。\n", r.From.UTC().Format(time.RFC3339), r.To.UTC().Format(time.RFC3339), len(records.Samples), baseline, failures, unknown, records.Truncated, eventTruncated, len(switches), len(incidents))
	files := []struct {
		name string
		data []byte
	}{{"summary.md", []byte(summary)}, {"samples.csv", csvBytes.Bytes()}, {"incidents.json", incidentData}, {"switches.json", switchData}, {"manifest.json", manifestData}}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	total := 0
	for _, file := range files {
		total += len(file.data)
		if total > 8<<20 {
			archive.Close()
			return nil, fmt.Errorf("诊断包超过8MiB，请缩短时间范围")
		}
		w, e := archive.Create(file.name)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(file.data); e != nil {
			return nil, e
		}
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
