# Live interface tour

[Project overview](../README.md) · [简体中文](screenshots.zh-CN.md) · [Capture provenance](assets/screenshots/README.md#live-20260911)

These English interface screenshots were captured in Microsoft Edge on Windows
on **2026-09-11**, from the running **NanoPi R5S LTS / ARM64, iStoreOS 24.10.8**
deployment, with frontend assets matching UI commit
`b0570dcf999d5f0445df06a61e3abed2681f2396`. The preview build was deployed before
that commit was created; the [manifest](assets/screenshots/live-20260911.json) records its provenance.
They show the application using its existing router data. Each image opens at
its original resolution when clicked.

All 18 captures use the light theme, a **1440 × 1000** CSS-pixel viewport and
native PNG output at device scale 1. Text and controls remain readable without
resizing the captured images. Only navigation, scrolling, the language picker,
opening advanced filters and an existing node-trend link were used;
no scans, retests, switches or configuration saves were triggered. Background
monitoring continued, so timestamps and measurements can differ between the
English and Chinese images. Node, provider and group identifiers keep their
original text; they are not interface translations.

| View | What to look for |
| --- | --- |
| [Scan workbench](#scan-workbench) | Saved results, ranking, sample evidence and expiry |
| [Expanded scan filters](#scan-filters) | Available regions, multiple selection and control spacing |
| [Monitoring overview](#monitoring-overview) | Current health, historical score and coverage |
| [Node details](#node-details) | P50/P95 trends, observation series and event markers |
| [Event timeline](#event-timeline) | Provider hints, event filters and loaded-record scope |
| [Monitor settings](#monitor-settings) | Existing failover state and diagnostic entry points |
| [Node catalog](#node-catalog) | Search, sorting, inferred regions and pagination |
| [Switch history](#switch-history) | Existing operations and Controller confirmation |
| [Runtime settings](#runtime-settings) | Scan budgets, result validity and optional verification |

<a id="scan-workbench"></a>
## Scan workbench

[![English scan workbench with existing results and an expired candidate comparison](assets/screenshots/scan-workbench-live-20260911-en.png)](assets/screenshots/scan-workbench-live-20260911-en.png)

The selected stable scan was recorded on **2026-09-09** and contains 46 candidates.
The screenshot date is later than the measurement date. “Results expired” and
disabled selection controls remain visible; historical timings are not presented
as current measurements. Search stays above the table, and candidate evidence
appears beside the ranking.

<a id="scan-filters"></a>
## Expanded scan filters

[![English advanced scan filters with separate region buttons and clear selected states](assets/screenshots/scan-filters-live-20260911-en.png)](assets/screenshots/scan-filters-live-20260911-en.png)

The region list reflects regions present in the node catalog, with Japan selected
in this saved scan. Separate buttons have clear gaps, selected checkmarks and
space to wrap. The active/recent scan label is separated from its dropdown. The
sidebar footer links to the GitHub project in a new tab. Opening the panel does
not start a scan or save a binding.

<a id="monitoring-overview"></a>
## Monitoring overview

[![English monitoring overview with current selection, attention summary and 24-hour ranking](assets/screenshots/monitoring-overview-live-20260911-en.png)](assets/screenshots/monitoring-overview-live-20260911-en.png)

The visible rows show a full 24-hour observation span and “Sufficient data”.
Coverage measures whether baseline slots have observations; success rate
measures successful probes. A healthy node can still have historical failures
and a lower score. Automatic failover is enabled in this particular deployment;
it is opt-in, and its candidate ordering is independent of the long-term score.

<a id="node-details"></a>
## Node details

[![English node history with observation evidence, P50/P95 trends and related event markers](assets/screenshots/monitoring-node-history-live-20260911-en.png)](assets/screenshots/monitoring-node-history-live-20260911-en.png)

The selected series is the current node, `日本aw3`. P50 is green and P95 is blue;
gray dashed vertical lines mark related events. A red hourly block means the
hour contains failures, not that the entire hour was offline. Gaps in the line
mean no successful samples were available for that interval, not zero latency.
The scoring window, sample coverage and observation span stay visible above
the chart.

<a id="event-timeline"></a>
## Event timeline

[![English event timeline with provider correlation explanation and loaded event filters](assets/screenshots/monitoring-events-live-20260911-en.png)](assets/screenshots/monitoring-events-live-20260911-en.png)

“No correlated incident is currently confirmed” does not mean there were no
node failures. The displayed list has 50 loaded records, and its filters apply
to those loaded records. Load older events to extend the search. Existing event
IDs support detail and trend links; opening this page does not perform a retest.

<a id="monitor-settings"></a>
## Monitor settings

[![English monitoring settings with failover enabled and collapsed diagnostic controls](assets/screenshots/monitoring-settings-live-20260911-en.png)](assets/screenshots/monitoring-settings-live-20260911-en.png)

The existing plan has automatic failover enabled. The screenshot records that
setting without changing it. Storage retention and diagnostic export sections
are collapsed, so this image documents their entry points, not an export or
particular retention values. A change in group selection does not establish
that existing connections migrated.

<a id="node-catalog"></a>
## Node catalog

[![English node catalog with search, natural sorting, inferred regions and bottom pagination](assets/screenshots/node-catalog-live-20260911-en.png)](assets/screenshots/node-catalog-live-20260911-en.png)

The deployment has **309 entries**, with **299** matching the default proxy-node
scope and **10** built-in outbounds or possible subscription notices hidden.
These are snapshot counts, not capacity limits. The screenshot shows the first
page of 25 entries, with an internal scroll area. Region names are localized;
inferred regions are not verified exit locations. Natural name order follows
the selected interface locale, so the two language captures may order names
differently.

<a id="switch-history"></a>
## Switch history

[![English switch history showing existing automatic switches and confirmed readback](assets/screenshots/selection-history-live-20260911-en.png)](assets/screenshots/selection-history-live-20260911-en.png)

The visible records are previous automatic monitoring switches. “Confirmed”
reports Controller readback for the target group selection. No manual switch
was performed for these screenshots, and the records do not demonstrate account
login, streaming access or long-lived connection continuity.

<a id="runtime-settings"></a>
## Runtime settings

[![English runtime settings showing scan budgets and optional exit checks](assets/screenshots/preferences-live-20260911-en.png)](assets/screenshots/preferences-live-20260911-en.png)

The page shows the deployment's stored scan parameters and optional verification
controls. These values are not advertised as universal defaults. Connection
secrets remain in the server configuration and are not shown. The page was scrolled to Runtime settings so the connection editor remains
outside the capture. Further settings continue below the visible viewport. Nothing was
saved or cleaned up during capture.

For detailed behavior, see the [monitoring guide (Chinese)](monitoring.md) and
[region guide (Chinese)](region-classification.md). The [capture manifest](assets/screenshots/live-20260911.json)
records file dimensions, capture times and SHA-256 hashes for this batch.
