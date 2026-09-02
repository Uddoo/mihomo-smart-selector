# Changelog

All notable user-visible and operator-visible changes to Mihomo Smart Selector
will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project intends to use [Semantic Versioning](https://semver.org/) once
versioned releases begin.

## [Unreleased]

### Added

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
