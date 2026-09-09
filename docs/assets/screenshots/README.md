# Documentation screenshots and provenance

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
for historical references. The README labels its older preferences view; the
current node-catalog showcase uses the dated live capture below.

<a id="live-20260909"></a>
## Maintainer-supplied live captures — 2026-09-09

These five screenshots were supplied by the maintainer for project documentation
from the running deployment. They are byte-for-byte copies of the supplied
3038 × 1886 PNGs: no crop, resizing, repainting or metric replacement was applied.
They retain the node/provider names and measurements visible in the supplied images.
The source commit is not encoded in the screenshots; the date describes this
capture batch, not a promise that future UI or runtime state will remain identical.

| File | View | Documentation use |
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
