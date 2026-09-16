# Long-running operation and recovery

## Controller destinations and credentials

Connection tests and saved GUI overrides can use loopback, RFC 1918 IPv4 and
IPv6 private addresses. Link-local, unspecified, multicast, public and known
cloud metadata destinations are rejected, including when returned by DNS.
Every new connection validates all DNS answers and dials a checked IP directly.
Controller requests ignore environment HTTP proxies and never follow redirects.
For a public or otherwise exceptional Controller, explicitly set its full URL
in `mihomo.controller` in the operator-owned YAML and restart the service. A GUI
save does not authorize additional public destinations.

Existing secrets are bound to their original scheme, hostname, port and base
path. After changing an address, choose **Enter a new secret** or **No secret**.
The **Server secret** option is only usable at the YAML Controller address.
An older saved override that reused the server secret at a different address
will be rejected at startup. Before upgrading such a deployment, configure that
Controller URL in YAML or save an explicit custom secret for the override.

LAN API tokens stay only in page memory. Reloading requires entering the token
again; scans and monitoring continue on the server. Older token entries are
removed from browser session/local storage when the page starts.

## Frontend loading, drafts and caching

Lazy-loaded pages show a loading state, then a recoverable error if their module
download fails or exceeds 15 seconds. **Retry loading** tries the module again;
if the browser remembers a failed module URL or the service has just upgraded,
use **Reload page** to fetch the current HTML and asset URLs. Reloading does not
start a scan or repeat a node selection.

Unsaved runtime and connection settings prompt before leaving the page,
refreshing or replacing the draft. Cancel preserves the form; saving successfully
clears its warning. A draft is not an automatic save or a browser backup: new
connection secrets stay in page memory and are cleared on confirmed departure.
Browser refresh/close prompts depend on browser support and user interaction.

Production builds include precompressed gzip copies of JavaScript and CSS. The
embedded server selects gzip using `Accept-Encoding`, sends `Vary`, and uses a
different strong ETag for each representation. Compression happens during the
build, not per request on the router. A pinned JavaScript compressor and zero
timestamps make gzip output reproducible across build hosts. Fingerprinted JS,
CSS and WOFF2 assets use `public, max-age=31536000, immutable`; HTML and unversioned files use
`no-cache` with ETag revalidation. API and SSE caching behavior is unchanged.
When placing a reverse proxy in front, preserve these headers and avoid forcing
immutable caching on HTML or missing assets.

## Scan lifecycle

`GET /api/v1/scans` returns the 20 most recent scan summaries plus any other
in-memory active scans. Running scans appear first. Results are loaded through
`GET /api/v1/scans/{id}`. These routes use the existing CIDR/token checks.

The browser remembers its last scan ID in session storage and reconnects after
reload, restoring the group, profile, mode and filters. A new browser uses the
active/latest scan; the workbench also offers an active/recent scan picker.
Browser updates never start a new scan or repeat a selection.

Active scans use the authenticated SSE endpoint `GET /api/v1/scans/{id}/events`
with a Bearer header (the token is not placed in the URL). Node updates are
coalesced over 250 ms. The browser reads an authoritative snapshot when the
stream connects, every 15 seconds while connected, and on terminal events.
The event queue is bounded and has no replay IDs, so these snapshots also repair
missed events. Only a snapshot can mark a scan complete and enable selection.
Events received during a snapshot read are discarded and followed by another
snapshot; they are not replayed over potentially newer state.

The server sends SSE heartbeats every 15 seconds. Each stream write/flush has
a 10-second deadline, cleared while idle; ordinary endpoints keep their
30-second response deadline. Failed stream writes release the subscription.

When SSE is unavailable, snapshot polling starts at 3 seconds and backs off
on read failures, up to 30 seconds. Stream reconnection backs off from 1 to
30 seconds; a stream silent for 35 seconds is reopened. Snapshot requests time
out after 10 seconds. Hidden or offline browser pages suspend the subscription
and timers; becoming visible/online triggers resynchronization. Backend scans
continue independently. A failed refresh retains the previous results with a
warning that clears after a successful snapshot.

Other API reads also have a 10-second deadline covering headers and the full
body. Mutations allow 25 seconds and diagnostic downloads allow 30 seconds.
A mutation timeout does not prove failure and never triggers an automatic
retry: check the resulting state or switch audit, retaining the original
selection request ID for any retry.

Monitoring overview refreshes run every 5 seconds after successful reads and
back off to 10, 20 and 30 seconds after repeated failures. The page retains its
last snapshot and displays the last successful read time on failure. Hidden or
offline pages cancel overview reads; visibility or connectivity restoration
triggers an immediate refresh. Successful discovery responses populate the
workbench as they arrive, while scanning/selection remain gated until discovery
and preflight validation finish. Workbench discovery also cancels obsolete
reads on visibility/connectivity changes and refreshes immediately on return,
so a recovered monitor does not retain an old shell-level network error.

The node catalog and scan ranking render 50 items per page. Scan search and
current/candidate location controls operate over the full result set; pagination
does not change scoring, scan scope or selection validation. Desktop and mobile
render only their active list layout. Preferences and storage panels load when
first visited.

Before accepting requests, startup marks leftover `running`/`pending` scans as
`interrupted`. It does not resume their network probes. A clean shutdown blocks
new scans, cancels active contexts, waits for workers and allows completed
partial results to be persisted as cancelled. A hard kill can lose in-memory
partial samples; the durable scan row remains available as interrupted on boot.

`scanner.concurrency` (default 4) is a global node-probe budget shared by all
scans. `scanner.max_active_scans` (default 2, range 1..8) bounds simultaneous
scans. A group cannot have two running scans. Admission rejects excess work
instead of creating an unbounded queue. Isolated verification continues to
require a single scan because its selector/listener is shared.

## Selection operations

`POST /api/v1/scans/{id}/select` accepts a `request_id` (1..128 characters) in
addition to `node`. The browser preserves this ID for retries after an uncertain
response. Reusing it for the same selection returns the stored operation and
does not send another Controller PUT. Reusing it for a different scan/node is
rejected. Older callers that omit it receive a server-generated ID, but cannot
rely on deduplication if they retry without that ID.

The operation is written to SQLite as `pending` **before** changing the selector.
Failure to persist the intent prevents the PUT. The server then reads back the
Controller and records one of these outcomes:

| Status | Meaning |
| --- | --- |
| `pending` | Intent exists; final outcome has not been persisted. |
| `confirmed` | Readback observed the requested member. |
| `failed` | Readback after a successful PUT or explicit reconciliation observed a different member. |
| `unknown` | Readback failed, the group disappeared, or a failed PUT left an ambiguous outcome. |

If the Controller confirmed the change but the final audit update fails, the
response still reports that observed outcome with `audit_persisted: false` and
an explicit message. The pending row remains durable for reconciliation. The
HTTP response being successful does not itself mean the operation is confirmed:
clients must inspect `status` and `audit_persisted`.

All unresolved operations remain visible in `GET /api/v1/history`, in addition
to the requested number of recent terminal records. A group with an unresolved
operation cannot accept a new switch request. `POST /api/v1/history/{id}/reconcile`
reads current Controller state and updates the audit row without sending a PUT.
This observes current state; it does not prove which request caused a historical
change. Startup converts leftover pending intents to unknown and never replays
them. Legacy successful audit rows migrate to confirmed.

## Storage retention

Defaults, configurable in YAML and persisted through the preferences UI:

```yaml
storage:
  retention:
    scan_days: 30
    max_scans: 200
    audit_days: 180
    max_audit: 1000
```

An ended record is eligible once it exceeds either its age or count limit.
Running/pending scans, pending/unknown audit rows and scans referenced by those
unresolved rows are excluded, even if this exceeds the configured count.
Deleting a scan cascades to its sample/result rows; retained terminal audits
remain readable without those results. Bindings, settings and credentials are
not cleanup targets.

Maintenance runs at startup and hourly, skipping periods with active scans.
Cleanup and settings/selection operations are serialized. Removed rows are
followed by database compaction and a WAL checkpoint; an unchanged database is
not vacuumed on each interval. `GET /api/v1/storage` reports database/WAL bytes,
scan/audit counts and unresolved counts. The preferences page offers manual
cleanup with a confirmation showing the saved retention limits.

`POST /api/v1/storage/cleanup` requires `{"confirm":true,"revision":N}`, where
N is the current settings revision. A policy edit after confirmation preparation
causes rejection and requires a fresh confirmation. Existing saved settings
without these fields inherit YAML/default values. Retention applies to existing
history on the next service start or maintenance run.

For backup or downgrade, stop the service and preserve the database and any WAL
alongside the binary and configuration. Reconcile unknown operations before
using a prior version that does not understand operation states.
