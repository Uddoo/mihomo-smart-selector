---
version: 1
slug: "web-src-connectivity-page-vue"
primary_target: "web/src/features/connectivity/ConnectivityPage.vue"
related_targets: ["web/src/features/connectivity/ConnectivityGroup.vue", "web/src/features/connectivity/ServiceCard.vue", "web/src/features/connectivity/ServiceViewControls.vue", "web/src/features/connectivity/ServiceBindings.vue", "web/src/features/connectivity/useConnectivity.ts", "web/src/features/connectivity/bindings.ts", "web/src/features/connectivity/useConnectivityBindings.ts", "web/src/App.vue", "web/src/app/useApplication.ts", "web/src/app/useFeatureNavigation.ts", "web/src/features/node-catalog/NodeCatalog.vue", "web/src/features/node-catalog/useNodeCatalog.ts"]
---

# Service connectivity

Scope: a separate top-level Connectivity tab. Mode: Operate.
Approved by the user's “开始实现” after the generated preview on 2026-09-15.
Current direction: the user's 2026-09-16 request replaces regional grouping with service types and refines presentation. The original regional preview is historical context.

## Direction contract

THESIS: Find common services by purpose, then inspect which respond from this browser, with eight visible samples per service.

OWN-WORLD: Inherit Geist typography, neutral surfaces, thin borders, existing 6/8px radii, Lucide controls and light/dark theme tokens. Service marks identify targets; colors describe measured outcomes with text equivalents.

STORY: Open the independent tab in its idle state, explicitly start a test, inspect per-service samples, refresh all or one group, stop an active test. Browser measurements are separate from node scanning and persistent monitoring.

FIRST VIEWPORT: Existing navigation and page heading; All services / My services; category counts and shared search; explicit test controls and scope feedback. Three-column service cards start with AI (6), then Social (10), Media (7), Development & cloud (4), Search & news (9), Shopping (6), Gaming (2), Tools (4). At <=1199px use two columns; <=640px use one. At <=760px a native type select replaces the category buttons alongside search.

FORM: Service purpose determines grouping. Existing app shell, palette tokens and evidence semantics remain authoritative. Category icons are neutral; active filters use the theme accent, running probes use information color, and result tones retain their meanings. Each card separates the name/reading from its sample strip and completed count. No list entrance or filter animations.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Component map: App composes the route; ConnectivityPage composes toolbar and groups; ConnectivityGroup accepts group/results/busy and emits refresh; ServiceCard accepts target/result and renders accessible evidence. useConnectivity owns lifecycle and state. Pure engine owns bounded requests, cancellation and measurements. No Controller mutations.

## Implementation record

2026-09-16 service types: The catalog replaces geographic `group` metadata with one explicit `category` per service, preserving every ID, URL and probe rule. `categories.ts` defines the eight typed categories. `ServiceFilters` receives categories/counts/query and emits filter changes; `useConnectivity` derives the intersection of view, category and translated-name/ID search before defining test scope. The issues filter holds active retests until completion. Existing observations, binding snapshots, cancellation and manual-start behavior remain intact. `ConnectivityGroup` adds category icons, counts and issue summaries; `ServiceCard` uses an 80px minimum height and separate sample row. Mobile controls and bilingual Nord/Geist captures are verified in `.impeccable/review/connectivity-categories/` and `.impeccable/review/connectivity/` with controlled responses, not live availability measurements.

2026-09-15: The six-entry navigation includes mobile **Links**. Service groups use four columns by default, three at 761–1250px, two at 481–760px and one at 480px or below. Shared Geist typography, theme tokens and control geometry remain the incumbent system. Native `details` / `summary` exposes sample results, times and probe addresses through click, tap or Enter / Space; Escape and the close button restore focus to the sample strip.

Service icons are local assets with sources and SHA-256 values in `docs/assets/service-icons.sources.json`; unavailable icons use Globe. Captures under `.impeccable/review/connectivity/` use controlled responses and do not establish live public-service speed or availability. This record documents the implementation; final workflow and review disposition are tracked separately.


2026-09-15 refinement: observations, issues-only preference and regional collapse state persist in the current browser document. Leaving stops work; returning shows retained evidence. A persistent toolbar status names the last/current scope, samples, total reachability and update time. Retest issues snapshots completed/stopped failures or median >=400ms, retains current work in the filtered view, and does not modify other observations. Cards expose one-service retesting. Empty/in-flight work is not failed evidence. Latency tone and partial-success warnings are independent.

ConnectivityToolbar accepts run/status/filter props and emits actions; ConnectivityPage only composes it with groups and explanatory copy. useConnectivity owns document-lifetime state and synchronous cancellation with stale-run guards. useSampleDetails owns open-only listeners, outside dismissal, focus return and viewport positioning. Region toggles remain visible with summary counts. Detail close and retest actions have 44px targets. Icons use selective dark-theme backing; Zoom's compact official favicon and Sony's enlarged proportional wordmark replace uniformly tiny marks. Source URLs and hashes remain in the provenance manifest.

2026-09-15 manual-start correction: entering, returning to or reloading Connectivity never starts probes. The first primary action is Start test; group/service actions can also start the first run. No reachability aggregate is shown before any test is requested. Existing cancellation and document-local result retention remain.

2026-09-15 binding extension: **By region / My services** adds a view control above the existing toolbar. My services includes only valid explicit profile bindings that match fixed catalog cards; Apple maps to `apple-services`. Multiple groups for one service remain separate in details, while manual testing samples that service once per run. Group refresh and issue retesting use the current view's scope. Entry, view changes, configuration refresh and selection changes never start probes.

`ServiceViewControls` presents the view, configuration read time, refresh action and manual-retest prompt. `ServiceBindings` extends the existing sample details with each bound group's configured selection, invalid/unknown states and latest matching group/profile scan from the available recent records. Its candidate action opens the existing catalog scoped to direct group members; its scan action opens an existing record. These actions do not start scans or switch nodes. The details state that configuration does not establish the browser request's actual path. The shared buttons, neutral selected surface, thin separators, warning text, wrapping labels and (44px) detail actions reuse the incumbent system without adding global tokens or an identity change.

`bindings.ts` resolves exact identities and selection changes; `useConnectivityBindings` accepts completed configuration reads and holds per-service test snapshots in document memory. `useConnectivity` keeps view scope and explicit test starts. App/useApplication and useFeatureNavigation supply read-only configuration and guarded navigation; NodeCatalog/useNodeCatalog apply and clear the group scope. A failed configuration read makes associations unavailable; the regional browser test remains available. Changing a configured selection after a test prompts manual retesting, and testing one service updates only its snapshot.

Binding documentation is in `docs/connectivity.md`. Captures under `.impeccable/review/connectivity-bindings/` cover Chinese light and English dark desktop/mobile layouts with controlled fixtures, not live service or router results. This record documents the extension and its existing design-system use; final validation and finish-review disposition remain separate.

2026-09-15 validation extension: card readings add a secondary evidence label (reachable, verified or resource reachable); readable mismatches and unverifiable checks remain explicit issues. `ProbeEvidence` adds target kind, expected status/content/body, redirect/cache policy and limits inside the existing disclosure. HTTP error responses can establish reachability without becoming successful samples. `NodeVerification` reads matching strict-scan evidence only on request, shows identity/rule/expiry issues, and offers a manual workbench preparation action. Browser and historical Go probe paths are named separately. Existing layout, semantic colors, bounded details, 44px actions and bilingual styling are retained; no new global design tokens or media are introduced. The rule catalog and scope are documented in `docs/connectivity.md` and `docs/connectivity-targets.md`.
