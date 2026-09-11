# Documentation screenshots and provenance

<a id="live-20260911"></a>
## Edge captures from the deployed router — 2026-09-11

The maintainer explicitly authorized this live capture batch. All **18 images**
were captured through **Microsoft Edge on Windows** from the running
**NanoPi R5S LTS / ARM64, iStoreOS 24.10.8** deployment with frontend assets matching UI commit
`b0570dcf999d5f0445df06a61e3abed2681f2396`. The preview was deployed before
committing; the manifest records its base commit, modified build state and binary
hash. All 18 served frontend assets match the subsequently committed files. English documentation uses the `-en`
images, while Chinese documentation uses `-zh-CN`.

Read the [English screenshot tour](../../screenshots.md) or
[中文实机导览](../../screenshots.zh-CN.md) for the displayed states and their meaning.

| View | English | 简体中文 |
| --- | --- | --- |
| Scan workbench | [English](scan-workbench-live-20260911-en.png) | [中文](scan-workbench-live-20260911-zh-CN.png) |
| Expanded scan filters | [English](scan-filters-live-20260911-en.png) | [中文](scan-filters-live-20260911-zh-CN.png) |
| Monitoring overview | [English](monitoring-overview-live-20260911-en.png) | [中文](monitoring-overview-live-20260911-zh-CN.png) |
| Node trends and history | [English](monitoring-node-history-live-20260911-en.png) | [中文](monitoring-node-history-live-20260911-zh-CN.png) |
| Event timeline | [English](monitoring-events-live-20260911-en.png) | [中文](monitoring-events-live-20260911-zh-CN.png) |
| Monitor settings | [English](monitoring-settings-live-20260911-en.png) | [中文](monitoring-settings-live-20260911-zh-CN.png) |
| Node catalog | [English](node-catalog-live-20260911-en.png) | [中文](node-catalog-live-20260911-zh-CN.png) |
| Switch history | [English](selection-history-live-20260911-en.png) | [中文](selection-history-live-20260911-zh-CN.png) |
| Runtime settings | [English](preferences-live-20260911-en.png) | [中文](preferences-live-20260911-zh-CN.png) |

The files contain native **PNG** bytes from Edge, captured at a **1440 × 1000**
CSS-pixel viewport and device scale 1 in the light theme. This smaller viewport
keeps text and controls readable in documentation. No image resizing, repainting,
redaction or metric replacement was applied. Runtime settings was scrolled into
view to keep the connection editor outside the capture. Dimensions, scroll
positions, capture timestamps and SHA-256 hashes are recorded in
[the manifest](live-20260911.json). The batch is approximately
2.09 MiB. These PNGs replace the earlier same-day JPEG batch.

The workbench shows an existing **2026-09-09** stable scan, including its real
expiry warning. Monitoring continued in the background during capture, so
timestamps, current states and measurements can differ between language pairs.
The catalog contains 309 entries, with 299 matching its default proxy-node scope;
these are snapshot counts. Interface text and region labels are localized, while
node, provider and group identifiers retain their original text. Natural sorting
can order names differently between locales.

Capture actions were limited to page navigation, scrolling, the language picker,
opening advanced filters and an existing node trend. No scan, retest, manual switch, configuration save,
cleanup or diagnostic export was triggered. The existing monitor plan and
failover setting were left in place. Access tokens, Controller credentials,
subscription URLs and the router's address are not visible in these images.
The task tab was closed after restoring the original Simplified Chinese locale
and resetting the temporary viewport override.

These images show runtime UI state, not an independent network benchmark,
streaming-unlock test or proof of long-lived connection continuity. The 24-hour
monitoring views report sufficient data; that label is not a 7-day reliability
study. Older mock and maintainer-supplied captures remain below as historical
assets, with their original provenance.

## Local mock showcase

The three mock showcase images below were captured from the real application at UI source commit
`ba501d0db4fd3163a640076838fd87cc4525eed7`, using `cmd/mihomo-mock` on temporary
loopback ports. No real provider credentials, subscriptions, private services or
production Controller are used.

| File | What it shows |
| --- | --- |
| `workbench-results.png` | Completed stable scan of five simulated nodes. ChatGPT group and ChatGPT profile match. |
| `node-comparison.png` | Unmodified crop of current/candidate metrics and sample evidence from that scan. |
| `switch-audit.png` | Two confirmed manual operations performed through the app against the mock. |

The fixture supplies fixed delays: the current member `JP-Osaka-02` reports
188 ms and the leading candidate `JP-Tokyo-03` reports 109 ms. These demonstrate
the comparison UI; they are not measured Internet performance or improvement
claims. Node names and providers are synthetic. README captions identify the
demo provenance. The main screenshot includes the application's own demo label.

`scan-workbench.png`, `node-catalog.png`, `selection-history.png`, and
`preferences.png` are earlier deployment captures. Existing images are retained
for historical references. Both READMEs now use the 2026-09-11 live batch above;
the mock comparison remains an input to the older sharing-cover artwork.

<a id="live-20260909"></a>
## Maintainer-supplied live captures — 2026-09-09

These five screenshots were supplied by the maintainer for project documentation
from the running deployment. They are byte-for-byte copies of the supplied
3038 × 1886 PNGs: no crop, resizing, repainting or metric replacement was applied.
They retain the node/provider names and measurements visible in the supplied images.
The source commit is not encoded in the screenshots; the date describes this
capture batch, not a promise that future UI or runtime state will remain identical.

| File | View | Original documentation use (historical) |
| --- | --- | --- |
| [monitoring-overview-live-20260909.png](monitoring-overview-live-20260909.png) | Monitoring overview, current selection and provisional 24-hour ranking | Both READMEs and monitoring guide |
| [monitoring-node-history-live-20260909.png](monitoring-node-history-live-20260909.png) | Observation series, history, coverage and event markers | Monitoring guide |
| [monitoring-events-live-20260909.png](monitoring-events-live-20260909.png) | Provider hint state and loaded incident timeline | Monitoring guide |
| [monitoring-settings-live-20260909.png](monitoring-settings-live-20260909.png) | Enabled failover, plan editing and collapsed diagnostic/storage controls | Monitoring guide |
| [node-catalog-live-20260909.png](node-catalog-live-20260909.png) | 299 visible entries out of 309, scope/status filters and inferred regions | Both READMEs and region guide |

The monitoring captures show about 14 hours of observation, not a completed
24-hour/7-day reliability study. Current healthy states and low historical
success rates describe different time scopes. Enabled failover is specific to
this deployment, and the switch notification is not evidence that existing
connections migrated. Collapsed settings show entry points, not successful
exports or particular storage policy values. Inferred regions are not verified
exit locations. Read [monitoring](../../monitoring.md#live-screenshots) and
[region classification](../../region-classification.md) for the interpretation.

Unlike the mock images, these live captures cannot be regenerated with
`capture-showcase.cjs`. Update them with a newly supplied or explicitly captured
live batch, use new dated filenames, and update captions/provenance together.
Keep older files while documentation still references them.

## Regenerate the mock showcase

Run `node tools/capture-showcase.cjs` from the repository with Go and an existing
Playwright installation available. If Playwright is outside the project, set
`MSS_PLAYWRIGHT_MODULE` to its module directory. Set `MSS_BROWSER_CHANNEL` to an
installed Playwright browser channel such as `msedge`, or leave it unset to use
Playwright Chromium.

The script builds the app and mock into ignored `.run/showcase`, creates a fresh
fixture database, drives actual scans and confirmations, captures the three
images and renders `../branding/social-preview.html` to `social-preview.png`.
It closes its browser and both child processes. Generated screenshot dates vary
between captures. It never contacts a production Controller.

The workbench uses a 1600 × 1040 CSS-pixel viewport at device scale 1.5; the
audit view uses 1040 × 780 so the text remains readable in a README. Crops exclude
unrelated controls and partial text; data and application styles are not edited.
The share cover is a deterministic HTML/CSS composition using the existing logo
and the comparison screenshot, with a visible `DEMO DATA` label.
