# Contributing to Mihomo Smart Selector

Thank you for helping improve Mihomo Smart Selector. The project accepts focused
bug fixes, tests, documentation, deployment hardening, and features that remain
inside its product boundary: compare eligible members of a user-selected Mihomo
`Selector`, keep scan selection explicit, and allow monitoring failover only
when the operator has enabled it.

Do not submit provider credentials, subscription URLs, Controller secrets,
private service addresses, complete proxy configurations, or logs that contain
them. Follow [SECURITY.md](SECURITY.md) for vulnerability reports.

<a id="your-first-contribution"></a>
## 第一次贡献 / Your first contribution

先从[首次贡献指南中的具体任务](docs/first-contribution.md#中文)选择一项，在 Issue 中说明意向。
欢迎中文和英文；一次只处理一个页面、一个说明或一组测试，无需先理解整个 Go/Vue 系统。

| 你的改动 | 从哪里开始 | 第一轮本地检查 |
| --- | --- | --- |
| 只改文档 | 对应的 `docs/` 指南；共享说明同步 [中文 README](README.md) 与 [英文 README](README.en.md) | 跟着修改后的步骤走一遍，在 Markdown 预览中检查链接、锚点和图片；修改截图时核对来源、语言、可读性和脱敏。运行 `git diff --check`。 |
| 只改前端或文案 | `web/src/`，翻译在 `web/src/i18n/messages.ts` | 按[本地 Mock 演示](README.md#development)启动模拟 Controller 与应用，只验证受影响页面的中英文、空状态和错误恢复；运行 `pnpm --dir web test`，然后 `pnpm --dir web build` 与 `./tools/check-web-assets.ps1`。 |
| 只改后端或测试 | 对应的 `internal/` 包 | 先运行相关包测试，例如 `go test ./internal/regions`；使用虚构数据，确认预期行为后再完成全量验证。 |

只提供[设备安装记录](https://github.com/Uddoo/mihomo-smart-selector/issues/new?template=installation_feedback.yml)
不需要编译源码：记录准确版本、架构、步骤、结果和未验证项即可。
截图规范见[截图来源说明](docs/assets/screenshots/README.md)。

**合并质量门槛保持不变。** 上表用于开始排查和验证；仍须完成下方的
[完整验证](#required-verification)，前端改动也必须保持内嵌资源一致。
第一次 PR 遇到检查失败时，请附命令、第一条有效错误、系统/架构及已完成的检查，可先提交草稿 PR 请求协助。
维护者可以协助解释失败、复现环境问题，或生成和复核需要维护者介入的构建产物。
未执行的检查应如实标记；合并前仍须通过规定检查，不删除检查、降低断言或手工伪造产物来换取通过。

Start with a scoped issue in the [first-contribution guide](docs/first-contribution.md#english).
One page, one setup step or one group of tests is enough for a first contribution.

| Your change | Start here | First local check |
| --- | --- | --- |
| Documentation only | The matching guide in `docs/`; update both READMEs for shared instructions | Follow the edited steps and check rendered links, anchors and images. For screenshots, verify provenance, language, readability and redaction. Run `git diff --check`. |
| Frontend or wording only | `web/src/`; translations are in `web/src/i18n/messages.ts` | Start the [local mock](README.en.md#development), verify the affected page in both languages including empty/error states, then run `pnpm --dir web test`, `pnpm --dir web build` and `./tools/check-web-assets.ps1`. |
| Backend or tests only | The relevant `internal/` package | Start with its tests, for example `go test ./internal/regions`, using fictional fixtures; then complete full verification. |

Installation reports need no source build. Record the exact version, architecture,
steps, outcome and untested areas. These first checks do not replace
[required verification](#required-verification) or embedded-asset consistency.
If blocked, open a draft PR with the failing command, first useful error, OS/architecture
and checks already completed. Maintainers can help interpret failures, reproduce
environment problems, or generate and review build artifacts needing maintainer support.
All required checks must still pass before merge; disclose unrun checks and keep
checks, assertions and artifact validation intact.

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
- Update both `README.md` (Chinese, the default) and `README.en.md` (English)
  when changing shared README contracts. Keep `README.zh-CN.md` as a compatibility
  entry point for existing links.
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

Browser reliability regressions run against a freshly built service and an
isolated mock Controller, with temporary SQLite storage and random loopback
ports. Install the locked Chromium build once, then run:

```powershell
pnpm --dir web exec playwright install chromium
pnpm --dir web test:e2e
```

The suite checks monitor snapshot replacement, incident focus, stale history
responses, request timeouts, retry backoff, offline/visibility recovery,
progressive discovery, and a lost switch response followed by an idempotent
retry after reload. Monitoring history uses deterministic API fixtures; the
scan/switch scenario exercises the real Go API, mock Controller and SQLite.
A real SSE connection also receives three 15-second heartbeats to check writes
beyond the server's ordinary 30-second response timeout; allow about a minute
for the suite.
Browser visibility transitions are simulated; offline transitions use the
browser context's network controls. CI installs Chromium with Linux system
dependencies and runs the same tests against the verified embedded assets.
Screenshots and failure traces go to the OS temporary directory under
`mihomo-smart-selector-e2e-results` (override with `MSS_E2E_OUTPUT`).

## Issue and pull-request flow

- Chinese and English reports are welcome. Start with the bilingual
  [troubleshooting guide](docs/troubleshooting.md), then choose Bug, Feature,
  or Other in the issue forms. Use Other for a private contact request with no
  sensitive evidence. Maintainer configuration is documented in
  [open-source setup](docs/open-source-setup.md).
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
