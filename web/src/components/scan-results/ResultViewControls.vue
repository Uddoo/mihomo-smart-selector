<script setup lang="ts">
import {computed} from 'vue'
import {t} from '../../i18n'
import {resultViews} from '../../scanResults'
import type {ResultView} from '../../scanResults'
const view = defineModel<ResultView>({required: true})
const order = computed(() => resultViews.find(item => item.id === view.value)!.order)
</script>

<template>
  <div class="result-view-controls">
    <div class="view-actions">
      <label class="view-label">{{ t('排序视图') }}<select v-model="view" class="view-select" :aria-label="t('排序视图')"><option v-for="item in resultViews" :key="item.id" :value="item.id">{{ t(item.label) }}</option></select></label>
      <slot/>
    </div>
    <p class="view-order" aria-live="polite">{{ t(order) }}</p>
    <p v-if="view !== 'score'" class="view-note">{{ t('排名列保留综合排名；查看顺序不改变评分、推荐候选或切换条件。缺测值排在有效测量之后。') }}</p>
  </div>
</template>

<style scoped>
.result-view-controls { margin-bottom: 16px; }
.view-actions { display: flex; gap: 12px; align-items: center; justify-content: space-between; flex-wrap: wrap; }
.view-label { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.view-select { min-height: 40px; max-width: 100%; padding: 8px 28px 8px 10px; border: 1px solid var(--control-border); border-radius: var(--radius-sm); color: var(--text); background: var(--surface); }
.view-order, .view-note { color: var(--muted); font-size: 12px; line-height: 1.6; margin: 8px 0 0; overflow-wrap: anywhere; }
</style>
