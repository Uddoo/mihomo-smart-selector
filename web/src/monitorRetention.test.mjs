import {test} from 'node:test'
import assert from 'node:assert/strict'
import {computed, reactive, ref} from 'vue'
import {retentionCapacity} from './monitorRetention.ts'

const defaults = {revision: 0, raw_days: 7, aggregate_days: 90, event_days: 90, max_raw_samples: 100000, max_hourly: 20000}

test('12 selected candidates disclose the 90-day shortfall without editing policy', () => {
  const policy = {...defaults}, result = retentionCapacity(12, policy)
  assert.equal(result.requiredHourly, 25920)
  assert.equal(result.hourlyShortfall, 5920)
  assert.equal(result.hourlyDays, 20000 / 288)
  assert.equal(result.rawDays, 7)
  assert.equal(result.requiredRaw, 75600)
  assert.deepEqual(result.recommendation, {max_raw_samples: 100000, max_hourly: 32000})
  assert.deepEqual(policy, defaults)
})

test('30 candidates model record capacity, not probe requests', () => {
  const result = retentionCapacity(30, defaults)
  assert.equal(result.rawPerDay, 23760)
  assert.equal(result.requiredRaw, 166320)
  assert.equal(result.requiredHourly, 64800)
  assert.equal(result.rawDays, 100000 / 23760)
  assert.equal(result.hourlyDays, 20000 / 720)
  assert.deepEqual(result.recommendation, {max_raw_samples: 200000, max_hourly: 78000})
})

test('preview follows actual selections and draft days/caps reactively', () => {
  const chosen = ref(6), policy = reactive({...defaults})
  const result = computed(() => retentionCapacity(chosen.value, policy))
  assert.equal(result.value.insufficient, false)
  chosen.value = 12
  assert.equal(result.value.insufficient, true)
  Object.assign(policy, result.value.recommendation)
  assert.equal(result.value.insufficient, false)
  assert.equal(policy.revision, 0)
  assert.equal(policy.aggregate_days, 90)
  policy.aggregate_days = 180
  assert.equal(result.value.requiredHourly, 51840)
  assert.equal(result.value.insufficient, true)
})

test('small intentional caps remain saveable inputs and are shown as capacity limits', () => {
  const result = retentionCapacity(12, {...defaults, max_hourly: 1000, max_raw_samples: 1000})
  assert.equal(result.hourlyDays, 1000 / 288)
  assert.equal(result.rawDays, 1000 / 10800)
  assert.equal(result.insufficient, true)
})

test('recommendations never reduce existing caps or exceed server limits', () => {
  assert.deepEqual(retentionCapacity(6, {...defaults, max_raw_samples: 800000, max_hourly: 80000}).recommendation,
    {max_raw_samples: 800000, max_hourly: 80000})
  const impossible = retentionCapacity(30, {...defaults, aggregate_days: 365})
  assert.equal(impossible.canFit, false)
  assert.equal(impossible.recommendation.max_hourly, 100000)
  assert.equal(retentionCapacity(30, {...defaults, aggregate_days: 130}).recommendation.max_hourly, 100000)
})

test('empty and invalid drafts do not produce NaN, Infinity or fictitious retention', () => {
  for (const count of [0, -1, 31, 1.5, NaN, Infinity]) assert.equal(retentionCapacity(count, defaults), null)
  for (const key of ['raw_days', 'aggregate_days', 'max_raw_samples', 'max_hourly']) {
    for (const value of ['', 0, -1, 1.5, NaN, Infinity]) assert.equal(retentionCapacity(12, {...defaults, [key]: value}), null)
  }
})

test('exact capacity is sufficient, one record short stays insufficient', () => {
  const policy = {...defaults, max_hourly: 25920, max_raw_samples: 75600}
  assert.equal(retentionCapacity(12, policy).insufficient, false)
  assert.equal(retentionCapacity(12, {...policy, max_hourly: 25919}).insufficient, true)
})

test('drafts outside server policy ranges do not offer unusable adjustments', () => {
  for (const change of [{raw_days:31}, {aggregate_days:366}, {aggregate_days:6}, {raw_days:8,aggregate_days:7},
    {max_raw_samples:999}, {max_raw_samples:1000001}, {max_hourly:999}, {max_hourly:100001}]) {
    assert.equal(retentionCapacity(12, {...defaults,...change}), null)
  }
})
