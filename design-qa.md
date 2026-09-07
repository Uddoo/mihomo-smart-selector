# Scan workbench design QA

Date: 2026-09-07

## Target and scope

- Reference: `docs/design/scan-workbench-v2-concept.png`, the approved Image Gen direction revised to include live ranking and per-node selection.
- Implementation capture: `docs/design/scan-workbench-v2-desktop.png`.
- Local preview: `http://127.0.0.1:18788`, real embedded Go/Vue application connected only to `cmd/mihomo-mock` on port 19090. Separate QA database and config under ignored local paths.
- Reference raster: 1487 × 1058; implementation CSS viewport: 1440 × 1024. Nearly identical aspect ratio. Both show five completed fixture results, JP-Tokyo-01 as current node and JP-Tokyo-03 as focused candidate.
- Both images were opened together for comparison. Browser screenshot scaling softens small text; DOM/AX inspection supplemented typography and copy checks. This is a layout/interaction review, not a pixel-equivalence claim.

## Findings and fixes

1. **P2, resolved: narrow-screen navigation wrapping and page overflow.** At 390px, no-wrap navigation initially forced document width to 406px. Added a bounded horizontally scrolling navigation and `min-width: 0` on its grid container. After rebuilding/reloading, document width was 375px within the 390px viewport (remaining space is the scrollbar). Navigation labels no longer wrap.
2. **P2, resolved: desktop detail proportions and dense table text.** Expanded the candidate panel to 380px and sidebar to 244px, increased table body to 14px and headings to 12px, retained 94px result rows. Final capture includes all five rows and the primary selection action.
3. **P2, resolved: mobile per-row action access.** Kept horizontal scrolling within the ranking table and made its action column sticky at the right edge. The final mobile capture shows a selection button alongside each visible result.

## Fidelity review

- **Typography:** Inter with Microsoft YaHei/system fallbacks; 28px page title, 18px section titles, 14px table content, tabular numerals. Smaller metadata is secondary. Native screenshot softness limits fine antialiasing comparison.
- **Layout:** compact configuration and collapsed probe details above one grouped ranking/detail workspace. Completed results and selection action remain within the desktop viewport. Mobile stacks candidate details below the table.
- **Colors:** retained blue/white system, neutral separators, subtle selected rows and separate semantic green/red/neutral statuses. Dark theme was inspected while live results were arriving.
- **Assets:** actual Lucide Vue components provide network and navigation icons. Minor icon-shape differences from the generated concept are intentional; no raster images are needed inside the product UI.
- **Copy:** live ranking, per-row selection, explicit confirmation, fixture indicator, and separate service verification states are present. Actual group labels come from the Controller, including its configured emoji. No unlock/login claims are introduced.

## Behavior evidence

- With a 3-second mock probe delay and concurrency 1, the UI displayed 1/5 results at 20%, then 4/5 at 80%. JP-Tokyo-03 (66.8, 109ms) moved ahead of JP-Tokyo-01 (63.7, 151ms) before completion.
- After explicitly focusing JP-Tokyo-01, arriving higher-scoring results changed row order without changing the candidate detail identity.
- All row selection buttons remained disabled during scanning and enabled for eligible results on completion.
- Selecting third-ranked JP-Tokyo-01 through its own row button opened the correct confirmation, changed the mock Controller, updated the current-node badge, and persisted switch history.
- Opening the fifth-ranked JP-Osaka-02 confirmation and pressing Escape left the current node unchanged and returned focus to its row action.
- Browser console inspection reported no warnings/errors during the verified flow.

## Automated verification

- Vue typecheck and production build passed; embedded assets match a fresh build.
- Three ranking tests passed: incremental insertion without snapshot mutation, backend-compatible stable tie breaking, and updated scores replacing stale rank fields.
- Go tests passed across all packages.
- Existing process smoke passed: discovery, preflight, scan, select and persisted history.
- Frontend dependency audit reported no known vulnerabilities.
- `git diff --check` passed.

## Boundaries

- Switching remains limited to completed scans, with the existing success-rate eligibility checks and server-side validation. Live ranking does not authorize switching an unfinished scan.
- Real router/provider performance, strict verification against live services, hosted CI, and human design acceptance were not exercised.
- No remaining P0/P1/P2 findings in the inspected scope. Small metadata sizing and icon differences are accepted P3 refinements rather than pixel-identical reproduction.

final result: passed
