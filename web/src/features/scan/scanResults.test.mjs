import assert from 'node:assert/strict'
import test from 'node:test'
import {
  orderResults,
  validity,
  sampleCounts,
  measurementSummary,
  buildResultExport,
  exportRows,
  csvCell,
  exportFields,
} from './scanResults.ts'
import { setLocale, t } from '../../i18n/index.ts'

const now = Date.parse('2026-09-15T10:00:00Z')
function node(name, rank, overrides = {}) {
  return {
    name,
    rank,
    provider: 'secret-provider',
    score: 80 - rank,
    success_rate: 1,
    stage: 'refined',
    p50_ms: 50,
    p95_ms: 100,
    jitter_ms: 0,
    samples: [
      { probe: 'private.example', delay_ms: 50 },
      { probe: 'private.example', delay_ms: 50 },
    ],
    measured_at: '2026-09-15T09:59:00Z',
    expires_at: '2026-09-15T10:09:00Z',
    strict_verification_status: 'not_checked',
    reachability_status: 'available',
    ...overrides,
  }
}
function snapshot(rows) {
  return {
    scan: {
      id: 'scan-1',
      status: 'complete',
      request: { target_group: 'private-group', mode: 'stable' },
      profile: {
        id: 'chatgpt',
        label: 'ChatGPT 服务可达性',
        transport_scope: 'HTTP latency only',
        targets: [{ address: 'https://private.example' }],
      },
      started_at: '2026-09-15T09:59:00Z',
      results: rows,
    },
    rows,
    selectable: rows.map((row) => row.name),
    at: now,
    view: 'score',
    filtered: false,
    demo: true,
  }
}
const defaults = {
  format: 'csv',
  fields: exportFields.map((f) => f.id),
  filter: 'all',
  includeNames: false,
}

test('view priorities and missing values preserve original ranks, evidence, and input order', () => {
  const rows = [
    node('missing', 1, { p50_ms: 0, p95_ms: undefined, samples: [] }),
    node('fast', 2, { p50_ms: 10, success_rate: 0.9 }),
    node('stable', 3),
  ]
  const before = structuredClone(rows)
  assert.deepEqual(
    orderResults(rows, 'stability', now).map((x) => x.name),
    ['stable', 'missing', 'fast'],
  )
  assert.deepEqual(
    orderResults(rows, 'response', now).map((x) => x.name),
    ['fast', 'stable', 'missing'],
  )
  assert.deepEqual(orderResults(rows, 'score', now), before)
  assert.deepEqual(rows, before)
  assert.equal(orderResults(rows, 'response', now)[0].rank, 2)
  const jitter = [node('no evidence', 1, { samples: [], jitter_ms: 0 }), node('measured zero', 2)]
  assert.equal(orderResults(jitter, 'stability', now)[0].name, 'measured zero')
  assert.deepEqual(
    orderResults([node('tie2', 2), node('tie1', 1)], 'response', now).map((x) => x.rank),
    [1, 2],
  )
})

test('verification ordering distinguishes passed, unknown, failed, expired and unknown expiry', () => {
  const rows = [
    node('failed', 1, { strict_verification_status: 'failed' }),
    node('unknown', 2),
    node('expired', 3, {
      strict_verification_status: 'passed',
      expires_at: new Date(now).toISOString(),
    }),
    node('fresh', 4, { strict_verification_status: 'passed' }),
    node('legacy', 5, { strict_verification_status: 'passed', expires_at: 'invalid' }),
  ]
  assert.deepEqual(
    orderResults(rows, 'verification', now).map((x) => x.name),
    ['fresh', 'legacy', 'expired', 'unknown', 'failed'],
  )
  assert.equal(validity({ expires_at: 'invalid' }, now), 'unknown')
  assert.equal(validity({ expires_at: new Date(now).toISOString() }, now), 'expired')
})

test('export aliases remain consistent across filters and exclude raw diagnostics and probe targets', () => {
  const rows = [
    node('private-a', 1),
    node('private-b', 2, {
      egress_error: 'raw-secret',
      samples: [{ probe: 'private.example', error: 'raw-secret' }],
    }),
  ]
  const data = snapshot(rows)
  data.selectable = ['private-b']
  const text = buildResultExport(data, { ...defaults, filter: 'selectable' })
  for (const secret of [
    'private-a',
    'private-b',
    'private-group',
    'secret-provider',
    'private.example',
    'raw-secret',
  ])
    assert.ok(!text.includes(secret), secret)
  assert.ok(text.includes('Node 2'))
  assert.ok(text.includes('Group 1'))
  assert.ok(text.includes('mock'))
  assert.ok(text.includes('snapshot_at'))
  assert.ok(text.includes('measurement_note'))
  assert.deepEqual(
    exportRows(data, 'selectable').map((x) => x.name),
    ['private-b'],
  )
  assert.ok(buildResultExport(data, { ...defaults, includeNames: true }).includes('private-b'))
})

test('CSV neutralizes formulas and quotes fields; Markdown escapes markup and table separators', () => {
  assert.equal(csvCell(' \t=SUM(A1)'), "' \t=SUM(A1)")
  assert.equal(csvCell('a,"b"\nc'), '"a,""b""\nc"')
  const data = snapshot([node('=HYPERLINK("https://example.com")', 1)])
  assert.ok(buildResultExport(data, { ...defaults, includeNames: true }).includes("'=HYPERLINK"))
  const markdown = buildResultExport(snapshot([node('<img src=x> | [link](x)\nname', 1)]), {
    ...defaults,
    includeNames: true,
    format: 'markdown',
  })
  assert.ok(!markdown.includes('<img'))
  assert.ok(markdown.includes('&lt;img src=x&gt; \\| \\[link\\](x)<br>name'))
  const hostile = '\\|\\[one]\\*two*\\|[three]\\'
  const escaped = '\\\\\\|\\\\\\[one\\]\\\\\\*two\\*\\\\\\|\\[three\\]\\\\'
  const combined = buildResultExport(snapshot([node(hostile, 1)]), {
    ...defaults,
    fields: ['name'],
    includeNames: true,
    format: 'markdown',
  })
  assert.ok(
    combined.includes('| ' + escaped + ' |'),
    'Backslashes and repeated Markdown metacharacters must all stay literal',
  )
})

test('export handles empty selections, partial scans, missing timing and translates surrounding labels', () => {
  const data = snapshot([
    node('legacy', 1, { expires_at: undefined, measured_at: undefined, p95_ms: 0, samples: [] }),
  ])
  data.scan.status = 'cancelled'
  data.selectable = []
  assert.equal(buildResultExport(data, { ...defaults, fields: [] }), '')
  assert.equal(buildResultExport(data, { ...defaults, filter: 'selectable' }), '')
  const text = buildResultExport(data, {
    ...defaults,
    fields: ['p95_ms', 'measured_at', 'expires_at', 'validity'],
  })
  assert.ok(text.split('\r\n')[1].startsWith(',,,unknown,'))
  assert.ok(text.includes('cancelled'))
  setLocale('en')
  try {
    assert.ok(
      buildResultExport(data, { ...defaults, format: 'markdown' }).startsWith('# Scan results'),
    )
    for (const field of exportFields)
      assert.ok(
        !/\p{Script=Han}/u.test(t(field.label)),
        `Untranslated export field: ${field.label}`,
      )
  } finally {
    setLocale('zh-CN')
  }
})

test('measurement summary counts only observed successful latency samples', () => {
  const rows = [
    node('a', 1),
    node('b', 2, {
      stage: 'screened',
      samples: [
        { probe: 'a', delay_ms: 0 },
        { probe: 'a', delay_ms: 99, error: 'failed' },
      ],
    }),
  ]
  assert.deepEqual(sampleCounts(rows[1]), { total: 2, successful: 0 })
  assert.deepEqual(measurementSummary(snapshot(rows).scan), {
    nodes: 2,
    samples: 4,
    successful: 2,
    min: 2,
    max: 2,
    refined: 1,
  })
  assert.deepEqual(measurementSummary(snapshot([]).scan), {
    nodes: 0,
    samples: 0,
    successful: 0,
    min: 0,
    max: 0,
    refined: 0,
  })
  assert.deepEqual(measurementSummary({ ...snapshot([]).scan, results: undefined }), {
    nodes: 0,
    samples: 0,
    successful: 0,
    min: 0,
    max: 0,
    refined: 0,
  })
})
