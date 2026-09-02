# Security policy

## Supported versions

Mihomo Smart Selector has not published its first release yet. Until a release
policy is documented, security fixes apply only to the latest commit on the
default branch. After releases begin, this section will list the supported
version series explicitly.

## Reporting a vulnerability

Do not report a suspected vulnerability in a public issue when the report
contains, or could reveal, a Mihomo Controller secret, API token, provider URL,
node credential, private service address, complete configuration, or other
sensitive network information.

For a public GitHub repository, use GitHub's private vulnerability reporting
feature when it is enabled. If that feature is unavailable, open a public issue
containing only a request for a private contact channel. Do not include exploit
details or sensitive logs in that issue.

A useful private report includes:

- the affected commit or release version;
- deployment platform and CPU architecture;
- Mihomo/OpenClash version and deployment mode;
- a minimal, redacted configuration;
- reproduction steps and the expected security boundary;
- impact and any known mitigation; and
- logs with secrets, provider URLs, node credentials, and private addresses
  removed.

The maintainer should acknowledge a private report, confirm whether it crosses
a documented security boundary, coordinate a fix and release, and only then
agree on public disclosure. No fixed response time is promised before the
project has a published maintenance policy.

## Security boundaries

The project treats the following as security-sensitive:

- exposing the Mihomo Controller secret to the browser, API, logs, database, or
  repository;
- accepting a non-loopback request outside the configured trusted CIDRs;
- accepting a non-loopback API request without authentication unless the
  operator explicitly enabled the high-risk unauthenticated-LAN mode;
- switching a Selector other than the one authorized by a completed scan;
- using the business Selector for strict or egress verification; and
- persisting provider URLs, node credentials, or complete proxy
  configurations.

Service reachability, latency, inferred node region, and an unauthenticated HTTP
status do not prove account login, subscription entitlement, streaming
availability, or regional unlock. Reports based only on those stronger claims
are product-semantics questions unless they also cross a security boundary.

## Operator responsibilities

Keep the web listener on loopback unless LAN access is required. For LAN access,
use a narrow trusted-CIDR allow-list and a high-entropy API token, keep config and
token files mode `0600`, never publish the service through a WAN port forward,
and review the active Mihomo/OpenClash configuration before enabling strict or
egress verification.
