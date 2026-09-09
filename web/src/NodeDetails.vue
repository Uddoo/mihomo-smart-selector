<script setup lang="ts">
import {ref, watch} from 'vue'
import {Copy, X} from '@lucide/vue'
import type {NodeSummary} from './models'
import {protocolLabel, providerLabel, regionSourceLabel, catalogRegionLabel, entryKindLabel, regionReasonLabel, regionEvidenceLabel} from './nodeCatalog'
const props = defineProps<{node: NodeSummary; regionLabel: (code?: string) => string}>()
defineEmits<{close: []}>()
const message = ref('')
watch(() => props.node, () => { message.value = '' })
async function copy(value: string, label: string) {
  try { await navigator.clipboard.writeText(value); message.value = `${label}已复制` }
  catch { message.value = '无法访问剪贴板，请选择完整文本手动复制。' }
}
</script>
<template>
  <div class="detail-heading"><h2 id="catalog-detail-title">节点详情</h2><button aria-label="关闭节点详情" @click="$emit('close')"><X :size="18"/></button></div>
  <h3>{{ node.name }}</h3>
  <button class="copy-action" @click="copy(node.name, '节点名称')"><Copy :size="15"/>复制节点名称</button>
  <dl>
    <dt>条目类型</dt><dd>{{ entryKindLabel(node.entry_kind) }}</dd>
    <dt>推断地区</dt><dd>{{ catalogRegionLabel(node, regionLabel) }}</dd>
    <dt>判断依据</dt><dd>{{ regionSourceLabel(node.region_source) }}</dd>
    <template v-if="node.region_candidates?.length"><dt>候选地区（待确认）</dt><dd>{{ node.region_candidates.map(code => regionLabel(code)).join('、') }}</dd></template>
    <template v-if="node.region_evidence?.length"><dt>命中名称线索</dt><dd>{{ node.region_evidence.map(value => regionEvidenceLabel(value, regionLabel)).join('、') }}</dd></template>
    <dt>节点来源</dt><dd>{{ providerLabel(node.provider) }}</dd>
    <template v-if="node.provider"><dt>完整来源标识</dt><dd class="raw-provider">{{ node.provider }}</dd></template>
    <dt>协议</dt><dd>{{ protocolLabel(node.protocol) }}</dd>
  </dl>
  <button v-if="node.provider" class="copy-action" @click="copy(node.provider, '来源标识')"><Copy :size="15"/>复制来源标识</button>
  <p class="region-note">{{ regionReasonLabel(node) }}</p>
  <p class="copy-message" role="status">{{ message }}</p>
</template>
<style scoped>
.detail-heading{position:sticky;top:0;z-index:1;background:var(--surface);padding-bottom:6px}
.detail-heading{display:flex;align-items:center;justify-content:space-between;gap:12px}.detail-heading h2{font-size:18px;margin:0}.detail-heading button{display:flex;align-items:center;justify-content:center;min-width:44px;min-height:44px}h3{font-size:20px;line-height:1.5;margin:20px 0 12px;overflow-wrap:anywhere}dl{margin:24px 0;font-size:14px}dt{margin-top:18px;margin-bottom:6px}dd{overflow-wrap:anywhere;line-height:1.6}.raw-provider{user-select:all}.copy-action{display:inline-flex;align-items:center;gap:8px;min-height:40px}.region-note,.copy-message{font-size:13px;line-height:1.8;margin:20px 0 0;color:var(--muted)}.region-note{padding-top:18px;border-top:1px solid var(--line)}.copy-message{color:var(--text)}
</style>
