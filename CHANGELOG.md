# Changelog

All notable user-visible and operator-visible changes to Mihomo Smart Selector
will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project intends to use [Semantic Versioning](https://semver.org/) once
versioned releases begin.

## [Unreleased]

### Fixed

- Drain workers and close result channels when a scan is cancelled; cancellation
  during isolated verification also prevents final ranking and completion.
- Use a fresh HTTP connection for each egress candidate to avoid reusing the
  previous node's connection, and restore the probe selector after cancellation.
- Award full jitter points for repeated identical successful samples; keep
  insufficient samples unscored and label them in the score breakdown.
- Use authenticated polling for dashboard scan progress instead of opening an
  EventSource connection without the required Bearer token.

### Added

- Independent node-catalog filters for region, provider and protocol, searchable
  metadata, natural sorting, removable filter chips and retained browsing state.
- On-demand node details with keyboard navigation and a mobile drawer, explicit
  region-inference explanations, copy actions, sticky headers and adjustable
  pagination with empty-state recovery.

- P2 monitoring: immutable plan revisions and independent observation series
  preserve history across candidate edits; P1 data migrates with a durable
  watermark without guessing missing legacy definitions.
- Exact hourly compaction and 1h/24h/7d views, history selection, latency trends
  and event markers. Automatic failover remains fixed to the 24-hour window.
- Provider correlation episodes, stable paginated incident timelines, configurable
  retention and storage reporting, and redacted diagnostic ZIP exports. Browser
  exports use a JSON envelope to avoid download-manager interception of fetch.

- Opt-in monitoring failover: confirmed current-node failure triggers selection
  by baseline success rate, then P95, among fresh healthy monitored candidates.
  Fresh verification, membership/readback checks, durable switch audits and a
  restart-safe two-minute cooldown protect the operation. A persistent page
  switch enables it; pausing monitoring also pauses failover.

- Opt-in persistent monitoring: one plan per Controller, up to six node/target
  combinations, two-minute baseline sampling, faster current-node checks,
  shared scan concurrency, bounded request rates and seven-day raw retention.
- A monitoring page with health transitions, state-change events, coverage,
  recent baseline history and provisional/ready 24-hour HTTPS scores. Browser
  closure does not stop monitoring; pause/resume and service restart retain
  evidence. Monitoring selects a production node only when failure-triggered failover is explicitly enabled.
- Transactional sample/state/event persistence and slot deduplication;
  additional retests cannot replace failed baseline samples. Controller errors
  and skipped work remain unknown instead of being charged to node failure.

- Startup recovery marks unfinished scans as interrupted and pending switches
  as unknown; active/recent scans can be restored after a browser reload.
- Durable switch intents, idempotent request IDs, Controller readback and an
  explicit reconciliation action for unknown outcomes; audit write failures
  after switching are distinguished from a failed switch.
- Configurable scan/audit retention with storage-size reporting, hourly
  maintenance and confirmed manual cleanup. Unresolved operations and their
  scans are retained.
- A global probe concurrency budget, a concurrent-scan limit and duplicate-group
  admission checks; graceful shutdown cancels scans and waits for their cleanup.

- Two-stage stable scans: screen all eligible nodes, then refine the top K and
  the eligible current member, with explicit sample evidence and timestamps.
- Result expiry and a single-node retest action that requires a new manual
  confirmation before switching; optional per-service strict/region gates.
- A two-second shared discovery cache for catalog reads; scans, binding writes,
  and selector membership validation continue to use fresh Controller data.

- Project logo in both READMEs and a simplified radar-and-node browser favicon.
- MaaEnd-inspired community configuration: bilingual issue entry points,
  troubleshooting guidance, an Other issue form, EditorConfig, issue label
  initialization/classification, and categorized GitHub release notes.
- Embedded Vue 3 dashboard and Go HTTP API.
- Mihomo Selector, Provider, and eligible leaf-node discovery.
- Configurable region aliases and node overrides.
- Service-aware Probe Profiles with separate reachability, strict verification,
  restriction, region-verification, and transport-scope states.
- Bounded quick/stable scans with progress, batching, median, P95, jitter,
  success rate, and explainable score components.
- SQLite scan results and manual Selector-switch audit history.
- Optional isolated strict-verification and actual-egress routes, disabled by
  default.
- Conservative OpenWrt/iStoreOS `procd` deployment template and rollback guide.
- MIT license, security policy, contribution guide, Code of Conduct, issue
  forms, and pull-request template.
- Deterministic frontend-asset verification and a cross-platform process-level
  smoke test using the development-only Mihomo mock.
- GitHub CI with Go race tests, dependency and secret scans, process smoke, and
  clean `linux/amd64` and `linux/arm64` builds.
- Dependabot configuration and a public-repository-gated CodeQL workflow; the
  private personal repository does not claim unavailable GHAS evidence.
- English and Simplified Chinese README entry points with real fixture-backed UI
  screenshots.

### Changed

- Probe execution, isolated verification, selection and lifecycle management
  have dedicated backend modules; frontend scan sessions and discovery are
  separated from the workbench view.

- Strict verification now checks the highest-ranked candidates within its
  budget instead of skipping the entire scan when the budget is exceeded.
- Stable rankings put refined candidates before screening-only results;
  screening-only candidates require a retest before selection.

- Redesigned the scan workbench with compact configuration, live sorted rankings,
  persistent candidate details, and per-node selection with explicit confirmation.
  In-progress results are ranked immediately; switching remains restricted to
  completed scans and eligible nodes.
- Canonical Go module path is now `github.com/Uddoo/mihomo-smart-selector`.
- Default branch is now `main`.
- Public router examples require both a generated API token and a narrow
  trusted-CIDR allow-list by default.
- Local development documents explicit Go, Node.js, and pnpm versions instead
  of assuming an untracked project-local Go toolchain.

### Security

- Router token files are forced to mode `0600` before service startup.
- Non-loopback examples fail closed when the API token is missing.
- Gitleaks scans both reachable Git history and the working tree.
- Govulncheck and pnpm audit run in the repository verification contract.
- GitHub Actions use read-only repository permissions, non-persistent checkout
  credentials, fixed runner/tool versions, and full commit-SHA action pins.

[Unreleased]: https://github.com/Uddoo/mihomo-smart-selector/commits/main
