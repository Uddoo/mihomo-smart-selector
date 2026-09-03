# Contributing to Mihomo Smart Selector

Thank you for helping improve Mihomo Smart Selector. The project accepts focused
bug fixes, tests, documentation, deployment hardening, and features that remain
inside its product boundary: rank eligible members of a user-selected Mihomo
`Selector`, and change that Selector only after an explicit decision.

Do not submit provider credentials, subscription URLs, Controller secrets,
private service addresses, complete proxy configurations, or logs that contain
them. Follow [SECURITY.md](SECURITY.md) for vulnerability reports.

## Development environment

Required tools:

- Go 1.27.x;
- Node.js 22.12 or newer;
- pnpm 11.19.x;
- PowerShell 7; and
- a POSIX `sh` implementation for validating the OpenWrt init script.

Install the locked frontend dependencies and download Go modules:

```powershell
go mod download
pnpm --dir web install --frozen-lockfile
```

For local UI work, start `cmd/mihomo-mock` and the real service with a copied
development config. Never commit the copied config or its SQLite data.

## Repository layout

```text
cmd/mihomo-smart-selector  application entry point
cmd/mihomo-mock            development-only Controller fixture
internal/api               HTTP API and embedded Vue application
internal/config            configuration, validation, and Probe Profiles
internal/history           SQLite scan and switch evidence
internal/mihomo            Mihomo Controller client
internal/regions           name-based region classifier
internal/scan              candidate, probe, score, and selection pipeline
web                        Vue 3 source and pnpm lockfile
deploy/openwrt             reviewed OpenWrt/iStoreOS template
tools                      build, verification, and process smoke scripts
```

## Make a focused change

- Keep unrelated changes out of the pull request.
- Preserve fail-closed behavior for authentication, candidate validation,
  strict verification, egress verification, and Selector writes.
- Do not turn reachability or an HTTP status into a claim about login,
  subscription entitlement, playback, or regional unlock.
- Add or update tests for changed backend behavior.
- Update both `README.md` and `README.zh-CN.md` when changing shared README
  contracts.
- Update `docs/architecture.md` when changing API routes, retained data, probe
  semantics, or security invariants.
- Update `CHANGELOG.md` for user-visible or operator-visible changes.

The Vue production build is committed under `internal/api/static` so a clean Go
checkout remains buildable. After changing `web/`, run:

```powershell
pnpm --dir web build
./tools/check-web-assets.ps1
```

Dependabot cannot regenerate committed Vite output. A Web dependency pull
request may therefore fail the embedded-asset check even when its source-level
types pass. A maintainer must check out that pull request, run the production
build, review the generated diff, and commit the matching
`internal/api/static` assets. Do not weaken the synchronization check to make an
automated dependency pull request green.

## Required verification

Before opening a pull request, run:

```powershell
./tools/verify.ps1
./tools/smoke-test.ps1
```

`verify.ps1` checks Go formatting, module consistency, vet, tests, frontend
types and production assets, shell syntax, GitHub Actions, dependency
vulnerabilities, and secret leakage. CI additionally runs it with `-Race` on
Ubuntu 24.04.

The process smoke test must prove the real service can discover the mock
Controller, preflight a scan, rank candidates, select a member, observe the
Controller change, and persist SQLite history. A unit test alone is not a
replacement for this process boundary.

## Issue and pull-request flow

- For non-trivial behavior changes, open an issue first and describe:
  - the operator scenario;
  - expected behavior before and after;
  - compatibility or deployment boundary impact.
- In pull requests, use the `.github` templates and include concrete verification
  results (including skipped checks, if any).
- If you change UI and backend together, include a screenshot, screenshot hash, or
  API evidence in the PR description.
- Always keep the verification and config steps in PR comments to make review
  repeatable by the maintainer.

## Commits and pull requests

Use focused conventional-style subjects, for example:

```text
fix(scan): reject stale selection results
test: cover provider healthcheck fallback
docs: clarify unauthenticated LAN risk
```

A pull request should state:

- the user-visible or operator-visible effect;
- the failure mode or design reason;
- security and compatibility impact;
- the exact verification commands and results;
- whether screenshots or generated web assets changed; and
- any remaining evidence boundary, such as missing real-router validation.

Do not report a test as passed if it was skipped, hung, or replaced by a
different command. GitHub Actions results complement local verification; they
do not prove real OpenWrt deployment or user mastery of the system.

## Review expectations

Maintainers may ask for a smaller scope, additional failure-path tests,
documentation in both README languages, or stronger evidence for changes that
touch secrets, CIDRs, proxy listeners, Controller writes, or automated
selection. A pull request can be technically correct and still be declined if
it expands the product boundary without a clear operational need.
