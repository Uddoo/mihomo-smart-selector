# Node naming audit — target router snapshot

## Scope and boundary

This is a point-in-time audit of **all provider-owned leaf-node display names**
returned by the target router's authenticated Mihomo Controller
`/providers/proxies` API on 2026-08-24. It does not retain provider URLs,
credentials, controller secrets, or raw subscription configuration.

The raw list is deliberately not committed: node names can expose provider
branding and change with every subscription refresh. The matching rules below
are derived from the complete live list and are kept as user-editable
`regions[].aliases` configuration.

## Observed shape

| Item | Count |
| --- | ---: |
| Unique provider leaf names | 310 |
| Providers represented | 14 |
| Japan name-inferred | 47 |
| Korea name-inferred | 12 |
| United States name-inferred | 45 |
| Hong Kong name-inferred | 63 |
| Other / not configured in v0.1 | 143 |

Examples establishing the compact country-code convention:

```text
JP-1      JP-2      JP-3      JP-4      JP-5      JP-6
JP1-HY2   JP2-HY2   JP3-HY2   JP4-HY2   JP5-HY2   JP10-HY2
KR-1      KR-2      KR-3      KR-4      KR-5
```

The prior alias matcher required a non-alphanumeric character immediately
after `JP`/`KR`; it therefore did not classify `JP4-HY2`, `JP1-HY2`, or `KR1`.
The current rule treats a two-letter country code as valid when it starts a
name/token and is followed by a digit, hyphen, underscore, or token boundary.
It still rejects accidental substrings such as `SJP`.

## v0.1 region coverage

Japan and Korea are the supported service-selection presets for this change:

- **Japan:** `JP`, Japanese labels, Tokyo/Osaka, and the Japanese flag.
- **Korea:** `KR`, `KOR`, `Korea`, `South Korea`, `Korean`, Seoul, Busan,
  Korean labels, and the Korean flag.

The Korea preset is a normal region chip in the UI. Selecting it filters to
Korean leaf nodes, including `KR1`/`KR-1` variants, and remains compatible with
manual scan-and-select safety gates.

## Broad-scan policy

“All” is no longer rejected merely because it exceeds the old 60-node cap.
The selector scans candidates in sequential batches of 60, with four concurrent
probes within each batch. The API exposes completed/total candidates plus
current/total batch number through both scan polling and SSE; the UI renders it
as a progress bar.

A separate `max_total_candidates: 500` remains as a fail-closed protection
against an unexpectedly huge subscription set.
