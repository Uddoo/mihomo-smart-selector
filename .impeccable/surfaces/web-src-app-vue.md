---
version: 1
slug: "web-src-app-vue"
primary_target: "web/src/App.vue"
related_targets: ["web/src/ScanWorkbench.vue","web/src/NodeCatalog.vue","web/src/MonitoringPage.vue","web/src/style.css"]
---

# Mihomo Smart Selector workbench

Scope: existing scan, monitoring, node catalog, switch history and settings routes.
Visitor mode: Operate. Build directly from the user's selected references in the existing Vue app.
This task does not establish a persistent build-path preference.

## Direction contract

THESIS: An evidence-first network workbench. Neutral navigation and compact task controls lead into a clear table, with comparison evidence adjacent to the inspected node.

OWN-WORLD: The user explicitly selected Geist and Carbon Data Table. Geist owns monochrome surfaces, typography, control geometry, spacing and restrained elevation. Carbon supplies table toolbar, explicit sorting, independent row actions and pagination conventions. Status colors retain existing meanings.

STORY: Select a service and group, scan, compare evidence, inspect a candidate, explicitly confirm switching, then verify the result. Monitoring keeps current health separate from historical evidence.

FIRST VIEWPORT: A 208px neutral sidebar and 24px work area. A 24px page heading pairs with language/theme controls. Compact configuration precedes a table toolbar and shared column rhythm. Search remains visible even with a short result set; comparison remains to the right on wide displays and in the established mobile drawer below 760px.

FORM: User-pinned canon, taking precedence over the direction roll (seed 2e3eb93a). References: https://vercel.com/geist/introduction and https://carbondesignsystem.com/components/data-table/usage/. The supplied third-party Vercel DESIGN.md is secondary visual evidence. Preserve bilingual copy, routes, API contracts, scoring and confirmation rules.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Component map: App owns navigation and presentation preferences; ScanWorkbench composes configuration, result toolbar, ranking and candidate details; a reusable table search control exposes only input/clear events; NodeCatalog owns its existing independent filters/sort/detail state; MonitorNodeList presents typed rows and emits inspect/retest. No new backend state or UI framework.

FINISH: 2026-09-11 — DESIGN.md and .impeccable/design.json recorded from the completed implementation; YAML/JSON, token references, 38 theme color values, narrative parity and LF formatting checked. The final reviewer disposition is ship for the two listed fixes (sample-data disclosure and documentation); this is not blanket certification of the whole surface. Evidence: .impeccable/review/finish-review.md.
