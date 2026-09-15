<script setup lang="ts">
import {t, formatDate} from '../../i18n'
defineProps<{view: 'regions' | 'mine'; count: number; changed: number; loading: boolean; available: boolean; updatedAt: number | null; locked: boolean}>()
defineEmits<{view: [value: 'regions' | 'mine']; refresh: []}>()
</script>

<template>
  <div class="service-view-controls">
    <div class="service-view-options" role="group" :aria-label="t('服务视图')">
      <button :aria-pressed="view === 'regions'" @click="$emit('view', 'regions')">{{ t('按地区') }}</button>
      <button :aria-pressed="view === 'mine'" @click="$emit('view', 'mine')">{{ t('我的服务') }}<span v-if="available">{{ count }}</span></button>
    </div>
    <div class="binding-refresh">
      <span v-if="loading" role="status">{{ t('正在读取策略组配置…') }}</span>
      <span v-else-if="!available" role="status">{{ t('策略组配置暂不可用') }}</span>
      <span v-else-if="updatedAt">{{ t('配置读取于 {time}', {time: formatDate(updatedAt, 'time')}) }}</span>
      <button :disabled="loading || locked" @click="$emit('refresh')">{{ t('刷新配置') }}</button>
    </div>
  </div>
  <p v-if="view === 'mine'" class="service-view-hint">{{ t('仅展示已有效绑定且在本面板有对应卡片的服务；测试仍从当前浏览器发起。') }}</p>
  <p v-if="changed" class="selection-change-hint" role="status">{{ t('{count} 项服务的配置选择已变化，请手动重测确认。', {count: changed}) }}</p>
</template>

<style scoped>
.service-view-controls { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:8px 16px; margin-bottom:12px; }
.service-view-options, .binding-refresh { display:flex; align-items:center; flex-wrap:wrap; gap:8px; }
.service-view-options button { display:flex; align-items:center; gap:8px; min-height:44px; }
.service-view-options button[aria-pressed="true"] { background:var(--soft); border-color:var(--text); }
.service-view-options span { color:var(--muted); font-variant-numeric:tabular-nums; }
.binding-refresh { font-size:12px; color:var(--muted); }
.binding-refresh button { font-size:12px; }
.service-view-hint, .selection-change-hint { margin:0 0 12px; font-size:12px; line-height:1.7; color:var(--muted); }
.selection-change-hint { color:var(--warning); }
@media (max-width:760px) { .binding-refresh button { min-height:44px; } }
</style>
