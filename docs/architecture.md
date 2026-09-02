# Mihomo Smart Selector — Architecture & deployment design

## 1. Decision and product boundary

This project is an **independent service**, not a fork or a plugin of
Zashboard, MetaCubeXD, OpenClash, or Mihomo. Its job is narrowly defined:

> For one user-selected Mihomo `Selector` group, rank the group's eligible
> members for a service probe and only change that selector after an explicit
> policy decision.

This preserves upstream dashboard upgrades, keeps the Mihomo secret out of the
browser, and makes every switch auditable. Mihomo already supplies the
controller operations needed to list groups, list providers, test a proxy, and
select a selector member. The project does **not** modify subscriptions,
decrypt provider files, train OpenClash Smart/LightGBM data, or make claims
about a provider's real location solely from a name.

The current locally-developed milestone is `v0.1`:

| Included now | Explicitly deferred |
| --- | --- |
| Vue dashboard, Controller integration, region/provider filtering, bounded scans, scoring, SQLite history, manual best-node selection | background scheduler, autonomous switching, long-connection/SSE quality probes, OpenClash LuCI integration |
| optional serialized egress verification through a dedicated hidden selector | automatic mutation/injection of the live OpenClash configuration |

This boundary is deliberate: a first router deployment must prove that discovery,
probe semantics, storage, and selector switching are correct before it is
permitted to change traffic unattended.

## 2. Security invariants

1. **Browser never calls Mihomo.** Only the Go process sends the controller
   `Authorization: Bearer` header.
2. **Controller remains local.** `external-controller` stays bound to loopback.
3. **Web server is loopback-only by default.** Any non-loopback deployment must
   require an API token and a trusted-CIDR allow-list; an unauthenticated
   `0.0.0.0` listener is forbidden.
4. **Secrets are environment-only.** The application config contains the name
   of an environment variable, never the secret itself. Subscription URLs and
   node credentials are neither persisted nor logged.
5. **Changing production traffic is explicit.** A scan has no effect on the
   user's service group. `POST .../select` validates that the chosen node was a
   member of the scanned selector before `PUT /proxies/{group}` is issued.
6. **Egress verification is isolated.** It may switch only a specially created
   hidden `__SMART_PROBE__` selector. Its listener binds `127.0.0.1`; the scan
   is serialized so concurrent requests cannot observe another candidate's
   egress.
7. **Failure fails closed.** Inability to reach Mihomo, an invalid selector,
   zero healthy candidates, or an invalid CIDR produces no switch.

For the verified `192.168.50.0/24` router deployment, the UI is available on
LAN port `8788` only with two safeguards: an allow-list for that CIDR and a
high-entropy token held in a mode-0600 router file. The static SPA shell is
public on that LAN so its login form can load; every `/api/` request, including
read-only data, requires the Bearer token. The browser retains the token only
in its own session storage.

## 3. Runtime architecture

```text
LAN browser / SSH tunnel
          │  (Selector REST + SSE; no Mihomo secret)
          ▼
┌─────────────────────────────────────────────────────────────┐
│ Mihomo Smart Selector (Go, default 127.0.0.1:8788)          │
│                                                             │
│ HTTP API ── Scan manager ── Score engine ── SQLite history  │
│                  │                 │                        │
│ Region classifier│                 └── switch audit         │
│                  ▼                                          │
│             Mihomo client                                   │
└──────────────────┬──────────────────────────────────────────┘
                   │ bearer secret, loopback only
                   ▼
          Mihomo Controller (127.0.0.1:9090)
                   │
        GET /proxies, /providers/proxies, /proxies/{name}/delay
        PUT /proxies/{selector}
                   │
                   ▼
      proxy providers / service endpoints

Optional actual-egress path (disabled by default):

Selector → PUT /proxies/__SMART_PROBE__ → 127.0.0.1:17890 mixed listener
        → HTTP request through listener → chatgpt.com/cdn-cgi/trace → loc=XX
```

The Controller API supports `GET /proxies`, `GET /providers/proxies`,
`GET /proxies/{name}/delay`, and `PUT /proxies/{name}` for selector choice.
Mihomo listeners also support a `proxy` field; a non-empty value routes that
listener directly to the named valid outbound or group. The official references
are [Mihomo Controller API](https://wiki.metacubex.one/en/api/) and
[listener common fields](https://wiki.metacubex.one/en/config/inbound/listeners/).

## 4. Candidate and score pipeline

```text
target selector
  │ normalized selector label → service Probe Profile
  │ profile label/semantics are persisted with the scan
  │ GET /proxies (selector.all, selector.now)
  ├──────────► reject unknown/non-member target
  ▼
members enriched with provider-name
  │ name aliases → manual override → unknown
  │ filter selected provider(s) + region(s)
  ▼
for every remaining member, bounded concurrent tests
  │ profile reachability endpoint × N samples
  │ provider leaves: reachable/timing only
  │ direct leaves: Mihomo evaluates expected HTTP code
  ▼
metrics: success %, p50, p95, mean adjacent jitter
  │ optional egress location (separate, serialized)
  │ optional strict status/body check through dedicated probe selector
  ▼
score / rank / persist immutable scan record
  │
  └── operator explicitly selects a result, or future scheduler passes policy
```

Only leaf proxies are candidates: nested `Selector`, `URLTest`, `Fallback`,
`LoadBalance`, `Smart`, and relay policy groups are excluded. This guarantees
that a displayed result is a direct member that the requested selector can
legally choose. A broad request is split into sequential batches of 60 by
default, while each batch keeps the controller probe concurrency bounded. Scan
polling and SSE expose completed/total candidates and current/total batch for
the UI progress bar. A separate `max_total_candidates` limit (500 by default)
still fails an unexpectedly huge request before it generates traffic. Quick
mode performs one sample per endpoint, while stable mode uses the configured
sample count.

### 4.1 Service probe profiles and verification boundaries

`scanner.probe_profiles` maps normalized Selector names to a read-only,
no-credential service profile. Decorations such as `🤖`, `📹`, spaces and `+`
are ignored during matching, so `🤖 ChatGPT` resolves to the `chatgpt` profile.
An unmatched group uses `scanner.default_probe_profile`. The scan API returns
the built-in public target addresses, expected status codes, and whether each
target is a reachability or strict check. Custom/private profile addresses (for
example an Emby hostname) remain masked to unauthenticated LAN browsers.

Each result reports five distinct facts rather than promoting a single HTTP
response into a stronger claim:

| Field | Meaning | Does not prove |
| --- | --- | --- |
| Reachability | bounded profile request reached the service surface | account login, subscription or stream playback |
| Strict verification | expected status and optional response text through the isolated probe route | general application behaviour beyond that assertion |
| Restriction | a configured restricted HTTP status was observed | a complete provider catalogue or every region's policy |
| Region verification | actual trace exit compared with explicitly configured expected regions | service licence entitlement |
| Transport scope | what the profile measured, normally HTTP latency only | TCP/UDP/QUIC game or video-stream quality |

The built-in Emby profile deliberately refuses to start a scan until an operator
sets a private endpoint using `scanner.probe_profile_overrides.emby`. This
changes that one profile without replacing the remaining built-ins.

Some OpenClash Smart controller builds expose provider-owned leaves only through
`/providers/proxies`, even though a Selector lists their names in `all`. The
scanner therefore merges that provider metadata before classification, while
retaining the Selector's own member list as the final write-back authorization
check.

For those provider-owned nodes, scanning uses the documented provider
`healthcheck` route rather than `/proxies/{name}/delay`; that endpoint accepts
the probe URL and timeout but not an expected response-status filter. Its score
therefore represents service reachability and timing. A profile requiring a
strict `401`, HTTP body or restricted-status assertion must instead use the
separate `scanner.strict_verification` route. That route is disabled by default
and selects candidates only in the explicitly configured, dedicated probe
Selector; it never changes the business Selector under evaluation. The optional
dedicated egress route remains the source of location verification.

The public API's `delay` result is a latency measurement, not an HTTP body.
Therefore the regular scan can verify expected HTTP response codes but cannot
determine the real exit country. Region aliases are labelled **name-inferred**.
Only the optional trace route may label an exit as **verified**.

The performance score is intentionally explainable and is returned both as a
total and as independent components:

| Component | Maximum | Meaning |
| --- | ---: | --- |
| Probe success | 40 | `40 × successful samples / all samples` |
| P95 latency | 20 | linearly decreases from the configurable P95 target |
| Median latency | 15 | linearly decreases from the configurable median target |
| Jitter | 10 | linearly decreases from the configurable jitter target |
| Egress confidence | 5 | verified matching exit only; 0 when unchecked/unknown/mismatched |
| **Total** | **90** | rounded to one decimal; service/region restriction state remains separate |

This is a ranking tool, not an assertion that a node will always be suitable for
interactive ChatGPT use. A successful `401` from `api.openai.com/v1/models`
is only an unauthenticated API boundary check; it is not an authenticated
service call. Strict checks never carry account tokens or send messages,
purchases, playback requests or other state-changing traffic.

## 5. API contract

| Method and route | Purpose | Side effect |
| --- | --- | --- |
| `GET /api/v1/health` | service and Controller reachability | none |
| `GET /api/v1/groups` | selectable Mihomo groups/current member | none |
| `GET /api/v1/providers` | available proxy providers | none |
| `GET /api/v1/regions` | configured classifier rules, no secrets | none |
| `POST /api/v1/scans` | submit group, regions, providers, mode | creates an inactive scan record only |
| `GET /api/v1/scans/{id}` | status and results | none |
| `GET /api/v1/scans/{id}/events` | live scan events (SSE) | none |
| `POST /api/v1/scans/{id}/select` | select ranked candidate | changes the specified selector after validation |
| `GET /api/v1/history` | past scan/switch evidence | none |

`POST /scans` body example:

```json
{
  "target_group": "🤖 ChatGPT",
  "regions": ["JP", "KR"],
  "providers": ["provider-a"],
  "mode": "stable"
}
```

No API returns the controller secret, provider subscription URL, proxy password,
or raw proxy configuration.

## 6. Data retained locally

SQLite at the configured path stores only operational evidence:

- scan identity, requested group/filters, start/completion time and outcome;
- result name, provider display name, inferred/verified region, samples and
  derived metrics; and
- selection audit: old member, new member, requested group, reason, timestamp.

It deliberately does not store the Mihomo `secret`, subscription URLs, or
complete provider/node configuration. Database growth needs a retention policy
before background scans are enabled.

## 7. Router integration (after local acceptance)

### 7.1 Read-only preflight

Before touching the target router, collect and preserve:

1. confirmed private or Tailnet destination and SSH host key fingerprint;
2. `ubus call system board`, free `/overlay` space, and available RAM;
3. `mihomo -v` and the active OpenClash/Mihomo controller address;
4. controller reachability from the router, **without printing its secret**;
5. the target group type and current member; and
6. a timestamped hash and backup of any OpenClash custom-override file that
   will be changed.

This project must not assume the router is ARM64. Build with the architecture
reported by the preflight, normally a CGO-free Go build for `linux/<arch>`.

### 7.2 Optional probe listener for strict checks

Only after the first manual scan/switch is accepted, install this reviewed
addition through OpenClash's supported custom override mechanism, adapted to
the active configuration syntax:

```yaml
proxy-groups:
  - name: "__SMART_PROBE__"
    type: select
    include-all: true
    hidden: true

listeners:
  - name: smart-probe
    type: mixed
    listen: 127.0.0.1
    port: 17890
    proxy: "__SMART_PROBE__"
```

The exact active config must be backed up and syntax-checked first. Do not add
this listener if port `17890` is occupied, if the target Mihomo version lacks
listener support, or if the config already defines the same names.

When it has been verified as an isolated listener, configure the application
explicitly and keep the candidate limit bounded:

```yaml
scanner:
  strict_verification:
    enabled: true
    selector_group: "__SMART_PROBE__"
    proxy_url: http://127.0.0.1:17890
    max_candidates: 60
```

The service disables keep-alive reuse while changing that selector and restores
its prior member when the strict phase finishes. Do not point this setting at a
selector carrying normal LAN traffic.

### 7.3 Service and rollback

Install the binary, config (mode `0600`), and data directory outside OpenClash
managed paths. A minimal `procd` service starts only after the network and
Mihomo are available; it restarts on crash but never resets Mihomo. Start on
loopback, verify the health endpoint, execute a manual scan, choose a node,
then verify the controller's `now` member.

Rollback is always: stop and disable the selector service → restore the backed
up override/config if one was added → restart OpenClash only when necessary →
verify the prior target group member and ordinary traffic. No subscription
provider changes are part of deployment.

## 8. Target-address gate

The confirmed deployment target is the private LAN router `192.168.50.2`.
The service is restricted to the configured LAN CIDR and must not be published
through a WAN port forward. The legacy `192.158.50.2` value is not a private
RFC 1918 address and must never be substituted for the confirmed router.
