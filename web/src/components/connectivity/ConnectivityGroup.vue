<script setup lang="ts">
import {Globe, RefreshCw, ChevronDown} from '@lucide/vue'
import {t, formatNumber} from '../../i18n'
import type {Target, ResultView} from '../../connectivity/engine'
import ServiceCard from './ServiceCard.vue'
import type {BindingMap} from '../../connectivity/bindings'

defineProps<{
  group: {id: string; name: string; code: string; targets: Target[]; visibleTargets: Target[]; active: boolean; reachable: number; partial: number; measured: number; average: number | null};
  results: Readonly<Record<string, ResultView>>; busy: boolean; collapsed: boolean; onlyIssues: boolean;
  bindings: BindingMap; available: boolean; loading: boolean; locked: boolean;
}>()
defineEmits<{refresh: [group: string]; retestService: [id: string]; toggle: [group: string]; candidates: [group: string]; scan: [id: string]; verify: [group: string, profile: string]}>()
</script>

<template>
  <section class="connectivity-group" :aria-labelledby="`connectivity-${group.id}`" :data-group="group.id">
    <div class="group-heading">
      <div class="group-summary">
        <Globe v-if="group.id === 'global'" class="group-globe" :size="24" aria-hidden="true"/>
        <svg v-else-if="group.id === 'cn'" class="group-flag" viewBox="0 0 30 20" aria-hidden="true"><rect width="30" height="20" rx="2" fill="#de2910"/><path d="m6 3 1 3h3L7.5 8l1 3L6 9l-2.5 2 1-3L2 6h3z" fill="#ffde00"/><g fill="#ffde00"><circle cx="12" cy="3" r="1"/><circle cx="15" cy="6" r="1"/><circle cx="15" cy="10" r="1"/><circle cx="12" cy="13" r="1"/></g></svg>
        <svg v-else-if="group.id === 'jp'" class="group-flag" viewBox="0 0 30 20" aria-hidden="true"><rect x=".5" y=".5" width="29" height="19" rx="2" fill="white" stroke="#d4d4d4"/><circle cx="15" cy="10" r="6" fill="#bc002d"/></svg>
        <svg v-else class="group-flag" viewBox="0 0 30 20" aria-hidden="true"><rect width="30" height="20" rx="2" fill="white"/><path d="M0 1h30M0 4h30M0 7h30M0 10h30M0 13h30M0 16h30M0 19h30" stroke="#b22234" stroke-width="1.5"/><path d="M0 0h13v11H0z" fill="#3c3b6e"/><path d="M2 2h1m2 0h1m2 0h1m2 0h1M2 5h1m2 0h1m2 0h1m2 0h1M2 8h1m2 0h1m2 0h1m2 0h1" stroke="white"/></svg>
        <h2 :id="`connectivity-${group.id}`" class="group-title"><button class="group-toggle" :aria-expanded="!collapsed" :aria-controls="`services-${group.id}`" :aria-label="t(collapsed ? '展开{group}分组' : '收起{group}分组', {group: t(group.name)})" @click="$emit('toggle', group.id)">{{ t(group.name) }}<ChevronDown :size="16" :class="{'is-collapsed': collapsed}" aria-hidden="true"/></button></h2>
        <span class="group-statistics">
          <template v-if="group.measured">{{ t('可达 {reachable}/{total}', {reachable: group.reachable, total: group.targets.length}) }} · {{ t('平均 {ms} ms', {ms: group.average === null ? '—' : formatNumber(group.average)}) }}<span v-if="group.partial"> · {{ t('{count} 项不稳定', {count: group.partial}) }}</span></template>
          <template v-else>{{ group.active ? t('测试中') : t('尚未测试') }} · {{ t('{count} 个服务', {count: group.targets.length}) }}</template>
        </span>
      </div>
      <button class="group-refresh" :disabled="busy" :aria-label="t('刷新{group}分组', {group: t(group.name)})" @click="$emit('refresh', group.id)"><RefreshCw :size="14" :class="{spinning: group.active}" aria-hidden="true"/>{{ t('刷新') }}</button>
    </div>
    <div :id="`services-${group.id}`" class="group-content" :hidden="collapsed">
      <template v-if="!collapsed">
      <div class="service-grid"><ServiceCard v-for="target in group.visibleTargets" :key="target.id" :target="target" :result="results[target.id]!" :busy="busy" :bindings="bindings[target.id] || []" :available="available" :loading="loading" :locked="locked" @retest="$emit('retestService', $event)" @candidates="$emit('candidates', $event)" @scan="$emit('scan', $event)" @verify="(group, profile) => $emit('verify', group, profile)"/></div>
      <p v-if="onlyIssues && !group.visibleTargets.length" class="group-empty">{{ t('本组暂无异常结果') }}</p>
      </template>
    </div>
  </section>
</template>

<style scoped>
.connectivity-group { margin-top:16px; }
.group-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:8px; min-height:32px; }
.group-summary { display:flex; align-items:center; gap:10px; min-width:0; flex-wrap:wrap; }
.group-toggle { display:inline-flex; align-items:center; gap:8px; min-height:44px; padding:4px 2px; border:0; background:transparent; font:inherit; }
.group-toggle .is-collapsed { transform:rotate(-90deg); }
.group-empty { margin:0; padding:8px 0; color:var(--muted); font-size:12px; }
.group-title { margin:0; font-size:17px; font-weight:650; line-height:1.4; }
.group-flag { width:26px; height:18px; flex-shrink:0; border-radius:2px; }
.group-globe { color:var(--blue); flex-shrink:0; }
.group-statistics { color:var(--muted); font-size:12px; font-variant-numeric:tabular-nums; }
.group-refresh { display:inline-flex; align-items:center; gap:6px; min-height:32px; padding:5px 10px; font-size:12px; flex-shrink:0; }
.service-grid { display:grid; grid-template-columns:repeat(4, minmax(0, 1fr)); gap:6px; }
.spinning { animation:connectivity-spin 1.5s linear infinite; }
@keyframes connectivity-spin { to { transform:rotate(360deg); } }
@media (max-width:1250px) and (min-width:761px) { .service-grid { grid-template-columns:repeat(3, minmax(0, 1fr)); } }
@media (max-width:760px) { .service-grid { grid-template-columns:repeat(2, minmax(0, 1fr)); } .group-refresh { min-height:44px; } .group-statistics { flex-basis:100%; } .group-summary { gap:6px 8px; } }
@media (max-width:480px) { .service-grid { grid-template-columns:minmax(0, 1fr); } }
@media (prefers-reduced-motion:reduce) { .spinning { animation:none; } }
</style>
