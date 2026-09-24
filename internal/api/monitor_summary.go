package api

import "github.com/Uddoo/mihomo-smart-selector/internal/model"

// Explicit fields shadow the embedded full representation. The legacy response
// keeps its original JSON contract, while summaries omit unrequested samples.
type monitorSummaryRow struct {
	model.MonitorRow
	Series *[]model.MonitorSample `json:"series,omitempty"`
}

type monitorSummary struct {
	model.MonitorOverview
	Rows           []monitorSummaryRow `json:"rows"`
	SampleSeriesID string              `json:"sample_series_id"`
}

func summarizeOverview(overview model.MonitorOverview, seriesID string) monitorSummary {
	out := monitorSummary{MonitorOverview: overview, Rows: make([]monitorSummaryRow, 0, len(overview.Rows)), SampleSeriesID: seriesID}
	for _, row := range overview.Rows {
		item := monitorSummaryRow{MonitorRow: row}
		// Only rows from this already-authorized task can supply samples. A
		// historical or foreign series never causes a cross-task history read.
		if seriesID != "" && row.SeriesID == seriesID {
			samples := row.Series
			item.Series = &samples
		}
		out.Rows = append(out.Rows, item)
	}
	return out
}
