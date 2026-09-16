<script setup lang="ts">
import {Bot, MessagesSquare, Clapperboard, CodeXml, Search, ShoppingBag, Gamepad2, PanelsTopLeft, RefreshCw, ChevronDown} from '@lucide/vue'
import {t, formatNumber} from '../../i18n'
import type {Target, ResultView} from '../../connectivity/engine'
import ServiceCard from './ServiceCard.vue'
import type {BindingMap} from '../../connectivity/bindings'
import type {CategoryID} from '../../connectivity/categories'

const categoryIcons = {ai: Bot, social: MessagesSquare, media: Clapperboard, development: CodeXml,
  information: Search, shopping: ShoppingBag, gaming: Gamepad2, tools: PanelsTopLeft}

defineProps<{
  group: {id: CategoryID; name: string; description: string; targets: Target[]; visibleTargets: Target[]; active: boolean; reachable: number; partial: number; issues: number; measured: number; average: number | null};
  results: Readonly<Record<string, ResultView>>; busy: boolean; collapsed: boolean; onlyIssues: boolean;
  bindings: BindingMap; available: boolean; loading: boolean; locked: boolean;
}>()
defineEmits<{refresh: [group: string]; retestService: [id: string]; toggle: [group: string]; candidates: [group: string]; scan: [id: string]; verify: [group: string, profile: string]}>()
</script>

<template>
  <section class="connectivity-group" :aria-labelledby="`connectivity-${group.id}`" :data-group="group.id">
    <div class="group-heading">
      <div class="group-summary">
        <h2 :id="`connectivity-${group.id}`" class="group-title"><button class="group-toggle" :aria-expanded="!collapsed" :aria-controls="`services-${group.id}`" :aria-label="t(collapsed ? '展开{group}分组' : '收起{group}分组', {group: t(group.name)})" @click="$emit('toggle', group.id)"><span class="category-icon"><component :is="categoryIcons[group.id]" :size="20" aria-hidden="true"/></span>{{ t(group.name) }}<span class="group-count">{{ group.targets.length }}</span><ChevronDown :size="16" :class="{'is-collapsed': collapsed}" aria-hidden="true"/></button></h2>
        <span class="group-description">{{ t(group.description) }}</span>
        <span class="group-statistics">
          <template v-if="group.measured">{{ t('可达 {reachable}/{total}', {reachable: group.reachable, total: group.targets.length}) }} · {{ t('平均 {ms} ms', {ms: group.average === null ? '—' : formatNumber(group.average)}) }}<span v-if="group.partial"> · {{ t('{count} 项不稳定', {count: group.partial}) }}</span></template>
          <template v-else>{{ group.active ? t('测试中') : t('尚未测试') }}</template>
          <span v-if="group.issues" class="group-issues"> · {{ t('{count} 项异常', {count: group.issues}) }}</span>
        </span>
      </div>
      <button class="group-refresh" :disabled="busy" :aria-label="t('测试{group}分组', {group: t(group.name)})" @click="$emit('refresh', group.id)"><RefreshCw :size="14" :class="{spinning: group.active}" aria-hidden="true"/>{{ t(group.active ? '测试中' : '测试本组') }}</button>
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
.connectivity-group { margin-top:24px; }
.group-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:12px; }
.group-summary { display:flex; align-items:center; gap:0 12px; min-width:0; flex-wrap:wrap; }
.group-toggle { display:inline-flex; align-items:center; gap:8px; min-height:44px; padding:0; border:0; background:transparent; font:inherit; }
.category-icon { display:grid; place-items:center; width:32px; height:32px; border-radius:var(--radius-sm); background:var(--surface-muted); color:var(--muted); }
.group-count { padding:2px 6px; border-radius:var(--radius-sm); background:var(--surface-muted); color:var(--muted); font-size:12px; font-weight:500; font-variant-numeric:tabular-nums; }
.group-description { font-size:12px; color:var(--muted); }
.group-issues { color:var(--warning); }
.group-toggle .is-collapsed { transform:rotate(-90deg); }
.group-empty { margin:0; padding:8px 0; color:var(--muted); font-size:12px; }
.group-title { margin:0; font-size:17px; font-weight:650; line-height:1.4; }
.group-statistics { flex-basis:100%; color:var(--muted); font-size:12px; line-height:1.5; font-variant-numeric:tabular-nums; }
.group-refresh { display:inline-flex; align-items:center; gap:6px; min-height:44px; padding:8px 12px; font-size:12px; flex-shrink:0; }
.service-grid { display:grid; grid-template-columns:repeat(3, minmax(0, 1fr)); gap:8px; }
.spinning { animation:connectivity-spin 1.5s linear infinite; }
@keyframes connectivity-spin { to { transform:rotate(360deg); } }
@media (max-width:1199px) { .service-grid { grid-template-columns:repeat(2, minmax(0, 1fr)); } }
@media (max-width:640px) { .service-grid { grid-template-columns:minmax(0, 1fr); } .group-description { display:none; } .group-summary { gap:4px 8px; } .group-title { font-size:16px; } }
@media (prefers-reduced-motion:reduce) { .spinning { animation:none; } }
</style>
