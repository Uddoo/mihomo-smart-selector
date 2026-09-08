# README showcase assets

The showcase is captured from the real application at UI source commit
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
for historical references. The expanded README gallery labels its older views.

## Regenerate

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
