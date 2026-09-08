# Long-running operation and recovery

## Scan lifecycle

`GET /api/v1/scans` returns the 20 most recent scan summaries plus any other
in-memory active scans. Running scans appear first. Results are loaded through
`GET /api/v1/scans/{id}`. These routes use the existing CIDR/token checks.

The browser remembers its last scan ID in session storage and reconnects after
reload, restoring the group, profile, mode and filters. A new browser uses the
active/latest scan; the workbench also offers an active/recent scan picker.
Polling never starts a new scan or repeats a selection.

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
