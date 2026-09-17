<p align="center">
  <img src="docs/assets/branding/social-preview.png" width="1280" alt="Mihomo Smart Selector — scan, compare, and confirm. Application preview uses demo data." />
</p>
<h1 align="center">Mihomo Smart Selector</h1>
<p align="center"><strong>Choose your next node with evidence.</strong></p>
<p align="center">A node comparison workbench for your existing Mihomo / OpenClash setup.<br>Screen all candidates, refine the shortlist, compare with your current node — then confirm the switch.</p>
<p align="center">
  <a href="#quick-start"><strong>Download &amp; run</strong></a> ·
  <a href="#showcase">See the workflow</a> ·
  <a href="#contributing">Contribute</a> ·
  <a href="README.md">简体中文</a>
</p>
<p align="center">
  <a href="https://github.com/Uddoo/mihomo-smart-selector/releases"><img src="https://img.shields.io/github/v/release/Uddoo/mihomo-smart-selector?sort=semver&amp;label=release" alt="Latest stable release" /></a>
  <a href="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml"><img src="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI status" /></a>
  <a href="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/codeql.yml"><img src="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/codeql.yml/badge.svg?branch=main" alt="CodeQL status" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT license" /></a>
</p>
<p align="center">
  <a href="#quick-start"><img src="https://img.shields.io/badge/desktop-Windows%20%7C%20macOS-0078D4" alt="Desktop: Windows / macOS" /></a>
  <a href="docs/first-contribution.md#english"><img src="https://img.shields.io/badge/PRs-welcome-brightgreen" alt="PRs welcome: first-contribution guide" /></a>
</p>

![English scan workbench on the router: saved results, ranked nodes, and an expired candidate comparison](docs/assets/screenshots/scan-workbench-live-20260911-en.png)
<p align="center"><sub>Live router deployment, captured in Microsoft Edge on 2026-09-11 at 1440 × 1000 for readable controls. The selected scan was recorded on 2026-09-09; its expiry warning is preserved. <a href="docs/screenshots.md">Browse eight views and the expanded scan filters in English.</a></sub></p>
<p align="center"><strong>Single binary · Manual scan selection · Opt-in monitoring failover</strong></p>

## Why try it?

| Your question | What the workbench gives you |
| --- | --- |
| **Which node should I use for this service?** | Choose a ChatGPT, YouTube, GitHub or custom probe profile and save its group binding. |
| **Is the candidate worth a closer look?** | Screen every eligible node, then refine the top K and your current node. Inspect P95, success counts and sample evidence side by side. |
| **What happened when I switched?** | Confirm explicitly, then check the Controller readback and audit status. Unknown outcomes have a reconciliation action. |
| **How does it behave over time?** | Monitor current health and historical coverage, inspect incidents, and optionally enable failure-triggered failover. |
| **Which common services can this network reach?** | The separate [Connectivity tab](docs/connectivity.md#english) tests 48 services from this browser, with regional groups, per-sample results and scoped refresh. |

The service works alongside Mihomo / OpenClash. It leaves subscriptions and the
business Selector unchanged during scans; optional strict/egress checks use a
separate probe Selector. The Controller secret stays on the backend.

<a id="quick-start"></a>
## Download and run

<p>
  <a href="https://github.com/Uddoo/mihomo-smart-selector/releases"><img src="https://img.shields.io/github/downloads/Uddoo/mihomo-smart-selector/total?label=asset%20downloads" alt="Total release asset downloads" /></a>
  <sub>Cumulative release asset downloads, including checksum files.</sub>
</p>

**Already using Mihomo / OpenClash / Clash Party? Start with a portable package.**
You need a running Mihomo instance, its HTTP Controller address and secret, and
a `Selector` group whose nodes you want to compare. Desktop packages need **no Go or Node.js**.

Open the **[latest stable release](https://github.com/Uddoo/mihomo-smart-selector/releases/latest)**
and expand **Assets**:

| Your device | Choose this asset suffix | Installation guide |
| --- | --- | --- |
| Windows · Intel / AMD 64-bit | `windows-amd64.zip` | [Windows](docs/windows.md#english) |
| Windows on ARM | `windows-arm64.zip` | [Windows](docs/windows.md#english) |
| Mac · Apple Silicon (M series) | `darwin-arm64.tar.gz` | [macOS](docs/macos.md#english) |
| Mac · Intel | `darwin-amd64.tar.gz` | [macOS](docs/macos.md#english) |
| OpenWrt / iStoreOS router · Linux | Build for your device architecture | [Build, preflight, install and rollback](deploy/openwrt/README.md) |

<p>
  <a href="deploy/openwrt/README.md"><img src="https://img.shields.io/badge/router-OpenWrt%20%2F%20iStoreOS-00A2DF" alt="OpenWrt / iStoreOS router deployment guide" /></a>
  <sub>Build from source for your device architecture.</sub>
</p>

The full archive name also includes the release version. **Source code (zip/tar.gz)**
is for development. Linux currently uses the source-build path; the published
desktop packages do not run on routers. Check each release's notes and `SHA256SUMS.txt`.

1. **Launch.** Extract the whole archive into a writable folder. On Windows, run
   `mihomo-smart-selector.exe`; on macOS, run `start.command`. Keep its terminal
   open. First launch creates `config.yaml` beside the executable.
2. **Connect.** Open **[localhost:8788](http://127.0.0.1:8788)** → **Settings → Mihomo
   connection**. Enter the Controller address and secret, test and save, then
   choose **Restart service → Confirm restart**. This restarts this application's
   runtime. [Connection setup](docs/connection-settings.md#english)
3. **Compare.** Choose your business Selector and service profile, run a **Stable**
   scan, and compare the refined candidates with your current node. Confirm a
   selection when ready, then check its Controller readback in switch history.

**Connection tip:** when both programs run on the same computer, the Controller
address might be `http://127.0.0.1:9090`. Use the address configured in your client;
the HTTP/SOCKS/mixed proxy port serves a different purpose. For a router, use an
address reachable from the computer running this service. In Clash Party, find
the Controller under **Core settings (内核设置)**.
[Clash Party settings screenshot](docs/assets/screenshots/clash-party-controller-settings-20260911-redacted.png)

Packages are currently unsigned; the platform guides cover OS requirements,
startup approval and upgrades. Keep the default loopback listener for local use;
LAN deployment requires the documented access token and trusted-CIDR settings.

**Just exploring?** [Browse the screenshot tour](docs/screenshots.md), or run the
[local mock demo](#development) with simulated nodes and no subscription.

<a id="showcase"></a>
## From a scan to a decision

Scan results include multi-level sort views, measurement details, and CSV / Markdown export and copy
with a content preview. [Read the result guide](docs/scan-results.md#english).

| Step | What to look for |
| --- | --- |
| **Choose a service** | Use a ChatGPT, YouTube, GitHub or custom profile; bind it to your own group. |
| **Screen and refine** | Stable mode screens eligible nodes, then retests the top K and current node. Check success counts, P95 latency and measurement time together. |
| **Confirm and check** | Select explicitly. Switch history records pending intent and a confirmed, failed or unknown Controller outcome; unknown outcomes can be reconciled. |
| **Observe over time** | Enable a monitoring plan and keep the backend service running to collect history after closing the browser. Review health, coverage and incidents; failure-triggered failover is opt-in. |

<details>
<summary><strong>See monitoring and switch history</strong></summary>

![English monitoring overview showing health, 24-hour scores and coverage](docs/assets/screenshots/monitoring-overview-live-20260911-en.png)

*Live capture, 2026-09-11. Coverage and success rate measure different things.
The enabled failover setting belongs to this deployment; it is off by default.*

![English switch history with Controller confirmation](docs/assets/screenshots/selection-history-live-20260911-en.png)

*Existing monitoring switch records; capturing the page did not trigger a switch.
Confirmed group selection does not prove that existing connections migrated.*

</details>

[Browse all views](docs/screenshots.md) · [Advanced scan filters](docs/screenshots.md#scan-filters) ·
[Node catalog and inferred regions](docs/region-classification.md)

## Ready for longer sessions

- **Resume your view:** reload the page to reconnect to an active scan; interrupted tasks remain visible after a service restart.
- **Bound the work:** scans share a global concurrency budget, with same-group duplication checks and a simultaneous-scan limit.
- **Keep history manageable:** separate scan/audit retention limits, storage-size reporting and confirmed manual cleanup.
- **Keep selection deliberate:** result expiry, optional per-service strict/region gates, durable intents and Controller readback.

These controls run in a single Go service with an embedded Vue interface and
SQLite storage. Ordinary scans keep manual selection; monitoring has an opt-in failure-triggered failover mode.

**Persistent monitoring:** after you enable a plan, this application's backend
periodically requests probes through the Mihomo Controller and saves the results
to its configured SQLite database. On a router, Windows or macOS, closing the
browser leaves monitoring running as long as the host running this application
stays awake and its backend process stays running. Exiting the desktop program or
stopping this application's router service stops sampling. Connecting to Mihomo
on a router does not transfer this application's monitoring plan to that router.
Monitoring includes
1h/24h/7d analysis, persistent node histories, exact hourly compaction, coverage,
Provider correlation hints and redacted diagnostic bundles.
Scores require 100 valid baseline samples and stay provisional until a full scoring window
with at least 80% coverage is available for the selected scoring window. WebSocket continuity is not verified.
Optional failover replaces a confirmed-unavailable current node with the healthy
monitored candidate having the highest baseline success rate (lower P95 breaks
ties), after a fresh check. Switches have durable audits and a two-minute cooldown.
[Monitoring usage and limits →](docs/monitoring.md)
[Operation and recovery details →](docs/operations.md)

<a id="compatibility"></a>
## Verified environments & project status

**First stable release: `v0.1.0`.** This README follows `main`; downloadable builds follow their
[release notes](https://github.com/Uddoo/mihomo-smart-selector/releases). Changes
on `main` can arrive before a public package. Check your installed version before
relying on a newly documented behavior.

| Evidence | Scope |
| --- | --- |
| **Local runtime** | Windows, using the included mock Controller and browser workflow tests. |
| **Router runtime** | NanoPi R5S LTS / ARM64 with iStoreOS 24.10.8. The 2026-09-11 Edge captures show eight views in both interface languages with UI assets matching commit `b0570dc`, at 1440 × 1000. Screenshots document the displayed state, not end-to-end service or long-connection reliability. |
| **Build validation** | CI is configured to build Linux ARM64 and AMD64; use the live CI badge for the current result. Build success is not runtime validation on every device. |

The project is under active development. Configuration and API compatibility
may change before `v1.0.0`. The dashboard supports Simplified Chinese and English,
with a language picker in the page header. HTTP reachability does not prove login, playback
or regional unlock. Strict checks and egress verification are reported
separately from performance scoring.

Dedicated verification checks the selected node before and after probing. If
probe-group restoration fails or cannot be confirmed, a separate warning remains
in the scan history; check that group's current selection in the Controller.
Readback cannot eliminate all races with other clients or validate the listener's
routing configuration.

<a id="contributing"></a>
## Help shape the next release

<p>
  <a href="https://github.com/Uddoo/mihomo-smart-selector/stargazers"><img src="https://img.shields.io/github/stars/Uddoo/mihomo-smart-selector?style=flat&amp;logo=github&amp;label=Stars" alt="GitHub Stars" /></a>
  <a href="https://github.com/Uddoo/mihomo-smart-selector/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22"><img src="https://img.shields.io/github/issues/Uddoo/mihomo-smart-selector/good%20first%20issue?label=good%20first%20issue" alt="Open good first issues" /></a>
</p>

**Start with one device, one page or one test case.** Chinese and English are
welcome, and contributing does not require a long-term commitment.

| You can help with | A useful first contribution |
| --- | --- |
| **Trying it on your setup** | Report your OS, architecture, app/client versions, and where connecting or scanning succeeded or got stuck. |
| **Documentation and translation** | Clarify one setup step or review one page's empty states and error messages in both languages. |
| **Go tests** | Add a fictional node-name case for ambiguous region classification, with agreed expected behavior. |
| **Packaging and OpenWrt** | Help scope Linux release archives, checksums and device validation. This needs an implementation discussion first. |

[First-contribution guide and scoped ideas](docs/first-contribution.md#english) ·
[Open issues](https://github.com/Uddoo/mihomo-smart-selector/issues) ·
[Share installation feedback](https://github.com/Uddoo/mihomo-smart-selector/issues/new?template=installation_feedback.yml)

Search existing issues before starting; comment on a matching task or open one
to agree on scope. The ideas in the guide are starting points, not assigned issues
or delivery promises. [CONTRIBUTING.md](CONTRIBUTING.md) explains the development
and verification workflow. Use [SECURITY.md](SECURITY.md) for sensitive reports.

If this helps you choose a node, **Star the repository to keep it handy**.
Sharing a reproducible problem or a small improvement helps the project grow too.

<a id="docs"></a>
## Documentation

| I want to… | Read |
| --- | --- |
| Explore the current interface in English | [Live screenshot tour](docs/screenshots.md) |
| Run a portable Windows EXE | [Windows installation and upgrades](docs/windows.md#english) |
| Run on Apple Silicon or Intel Mac | [macOS installation and upgrades](docs/macos.md#english) |
| Scan a service whose group contains other groups | [Nested Selector navigation and scope](docs/nested-groups.md#english) |
| Use my own group names, services or private endpoints | [Service adaptation & persistent settings](docs/service-adaptation.md) |
| Install on a router | [Deployment, preflight and rollback](deploy/openwrt/README.md) |
| Read monitoring health, trends, events and failover settings | [English screenshot tour](docs/screenshots.md#monitoring-overview) · [Detailed guide (Chinese)](docs/monitoring.md) |
| Understand inferred, ambiguous or hidden catalog entries | [Region inference and entry types](docs/region-classification.md) |
| Understand scores, isolation or the API | [Architecture & verification boundaries](docs/architecture.md) |
| Understand restart recovery, audit states or cleanup | [Long-running operation](docs/operations.md) |
| Diagnose a problem | [Troubleshooting & reporting](docs/troubleshooting.md) |
| See what changed | [Changelog](CHANGELOG.md) |

<a id="faq"></a>
## Common questions

**I already use `url-test`. Why add this?** Mihomo's native
[`url-test`](https://wiki.metacubex.one/en/config/proxy-groups/url-test/) supports
a test URL, interval and switching tolerance; it may be enough for simple
automatic selection. This workbench adds a visible comparison workflow: staged
sampling, current-versus-candidate evidence, historical coverage and switch audits.
Use it when you want to understand and review a choice.

**Does it include proxies or subscriptions?** Bring your own Mihomo setup and
nodes. This is a companion workbench; it provides no subscription or proxy core.

**Will scanning switch my current business node?** No. Ordinary scans require an
explicit selection to change that group. Optional verification may temporarily
change its dedicated probe Selector, then attempts to restore it. Separately,
monitoring can switch a confirmed-unavailable current node when you explicitly
enable its failure-triggered failover mode.

**Why can a higher-scoring node appear below another?** Stable scans put refined
nodes first. Within the same stage, ranking uses score, then success rate, then
lower P95. A screening-only candidate can be retested before selection.

**Why is selection unavailable?** Check the explanation beside the action: the
scan may be incomplete, its result expired, the node removed, its success rate
below the threshold, a service gate unmet, or an earlier switch unresolved.

**Why is a service failing?** First check the Controller connection and selected
profile. For a private service, configure its actual endpoint using a profile
override. [Troubleshooting →](docs/troubleshooting.md)

<a id="development"></a>
<details>
<summary><strong>Local mock demo, source build and tests</strong></summary>

<p>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/Uddoo/mihomo-smart-selector/main?logo=go&amp;label=Go" alt="Go version declared in go.mod" /></a>
  <a href="web/package.json"><img src="https://img.shields.io/github/package-json/dependency-version/Uddoo/mihomo-smart-selector/vue/main?filename=web%2Fpackage.json&amp;logo=vuedotjs&amp;label=Vue" alt="Vue version range declared in package.json" /></a>
  <a href="web/package.json"><img src="https://img.shields.io/github/package-json/dependency-version/Uddoo/mihomo-smart-selector/dev/typescript/main?filename=web%2Fpackage.json&amp;logo=typescript&amp;label=TypeScript" alt="TypeScript version range declared in package.json" /></a>
</p>

### Local mock demo

You need **Go 1.27.x** and two terminals. The Vue dashboard is embedded; this
demo needs no Node.js, subscription or real Controller secret.

Terminal 1:

```sh
git clone https://github.com/Uddoo/mihomo-smart-selector.git
cd mihomo-smart-selector
go run ./cmd/mihomo-mock
```

Terminal 2, from the cloned repository:

```sh
go run ./cmd/mihomo-smart-selector -config config.dev.example.yaml
```

Open [localhost:8788](http://127.0.0.1:8788), choose English if needed and run a
Stable scan. **Mock timings are simulated**, not measurements of your network.
Ports 9090 and 8788 must be free; both commands work in PowerShell and POSIX shells.
First run downloads Go dependencies. Ctrl+C stops each process.

To run from source against your own Controller, copy `config.example.yaml` to
`config.yaml`, run `go run ./cmd/mihomo-smart-selector -config config.yaml`, then
follow the connection steps above. Keep copied configuration and data out of commits.

### Build and verify

Frontend development needs Node.js ≥22.12 and pnpm 11.19.x in addition to Go
1.27.x. A project-local Go toolchain may live in `.tools/go`; it is not included
in a fresh clone.

```sh
go mod download
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go vet ./...
go test ./...
go build -o bin/mihomo-smart-selector ./cmd/mihomo-smart-selector
```

On Windows, use `bin/mihomo-smart-selector.exe` as the build output.
Repository verification runs through PowerShell:

```powershell
./tools/verify.ps1
./tools/smoke-test.ps1
```

Verification covers formatting, modules, tests, frontend assets, workflow/shell
syntax, dependency vulnerabilities and secret checks. `-SkipSecurity` is for a
faster local loop; CI is configured to include the Go race detector. The process
smoke test uses an isolated local mock and cleans up its processes.

Please read [CONTRIBUTING.md](CONTRIBUTING.md), the [Code of Conduct](CODE_OF_CONDUCT.md)
and the [maintainer setup guide](docs/open-source-setup.md). Keep credentials,
subscription URLs and private endpoint details out of commits and public
reports. Use [SECURITY.md](SECURITY.md) for sensitive reports.

</details>

---

[MIT License](LICENSE) · [Report an issue](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose) · [简体中文](README.md)
