<script setup lang="ts">
import {computed, shallowRef, ref, nextTick, useTemplateRef, watch, onBeforeUnmount} from 'vue'
import {Download, Copy, X} from '@lucide/vue'
import {t, formatDate} from '../../i18n'
import type {NodeResult, Scan} from '../../models'
import {buildResultExport, exportFields, exportRows} from '../../scanResults'
import type {ExportField, ExportFilter, ExportFormat, ExportSnapshot, ResultView} from '../../scanResults'

const props = defineProps<{scan: Scan | null; rows: NodeResult[]; view: ResultView; filtered: boolean; demo: boolean; selectionReason: (row: NodeResult) => string}>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const preview = useTemplateRef<HTMLTextAreaElement>('preview')
const snapshot = shallowRef<ExportSnapshot | null>(null)
const format = shallowRef<ExportFormat>('csv')
const filter = shallowRef<ExportFilter>('all')
const includeNames = shallowRef(false)
const fields = ref<ExportField[]>(exportFields.map(field => field.id))
const message = shallowRef('')
const failed = shallowRef(false)
const copying = shallowRef(false)
const content = computed(() => snapshot.value ? buildResultExport(snapshot.value, {format: format.value, fields: fields.value, filter: filter.value, includeNames: includeNames.value}) : '')
const count = computed(() => snapshot.value ? exportRows(snapshot.value, filter.value).length : 0)
watch([format, filter, fields, includeNames], () => { message.value = ''; failed.value = false }, {deep: true})
function close() { dialog.value?.close() }
watch(() => props.scan?.id, close)
onBeforeUnmount(close)
async function open() {
  if (!props.scan || !props.rows.length) return
  const selectable = props.rows.filter(row => !props.selectionReason(row)).map(row => row.name)
  // Copy once: streamed updates cannot change a preview after it was inspected.
  snapshot.value = JSON.parse(JSON.stringify({scan: props.scan, rows: props.rows, selectable, view: props.view, filtered: props.filtered, demo: props.demo, at: Date.now()}))
  filter.value = 'all'; includeNames.value = false; message.value = ''; failed.value = false
  await nextTick()
  dialog.value?.showModal()
}
async function copy() {
  if (!content.value || copying.value) return
  const text = content.value
  copying.value = true
  try {
    await navigator.clipboard.writeText(text)
    failed.value = false; message.value = '结果已复制'
  } catch {
    failed.value = true; message.value = '无法访问剪贴板，已选中预览内容，请手动复制。'
    preview.value?.focus(); preview.value?.select()
  } finally { copying.value = false }
}
function download() {
  if (!content.value || !snapshot.value) return
  let url = ''
  try {
    const csv = format.value === 'csv'
    url = URL.createObjectURL(new Blob([csv ? '\uFEFF' : '', content.value], {type: csv ? 'text/csv;charset=utf-8' : 'text/markdown;charset=utf-8'}))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `scan-results-${new Date(snapshot.value.at).toISOString().replace(/[:.]/g, '-')}.${csv ? 'csv' : 'md'}`
    document.body.append(anchor); anchor.click(); anchor.remove()
    message.value = '已发起文件下载'; failed.value = false
  } catch {
    message.value = '无法下载文件，请复制预览内容。'; failed.value = true
  } finally { if (url) window.setTimeout(() => URL.revokeObjectURL(url), 1000) }
}
</script>

<template>
  <button class="export-trigger" :disabled="!scan || !rows.length" @click="open"><Download :size="15" aria-hidden="true"/>{{ t('导出 / 复制') }}</button>
  <dialog ref="dialog" class="result-export-dialog" :aria-label="t('导出扫描结果')" @close="snapshot = null">
    <template v-if="snapshot">
      <div class="export-heading"><h2>{{ t('导出扫描结果') }}</h2><button class="close-export" autofocus :aria-label="t('关闭导出')" @click="close"><X :size="18" aria-hidden="true"/></button></div>
      <p class="export-description">{{ t('固定打开时的结果快照，覆盖当前搜索的全部分页；预览、复制与下载使用相同内容。') }}</p>
      <p class="snapshot-time">{{ t('快照时间') }} {{ formatDate(new Date(snapshot.at)) }} <span v-if="snapshot.demo"> · {{ t('示例数据') }}</span></p>
      <p v-if="snapshot.scan.status !== 'complete'" class="export-warning">{{ t('扫描尚未完整结束，当前仅为部分结果。') }}</p>
      <div class="export-options">
        <label class="export-option">{{ t('文件格式') }}<select v-model="format" class="export-select"><option value="csv">CSV</option><option value="markdown">Markdown</option></select></label>
        <label class="export-option">{{ t('导出筛选') }}<select v-model="filter" class="export-select"><option value="all">{{ t('全部搜索结果') }}</option><option value="selectable">{{ t('快照时可选择的结果') }}</option><option value="refined">{{ t('已复测结果') }}</option></select></label>
      </div>
      <label class="name-choice"><input v-model="includeNames" type="checkbox">{{ t('包含真实节点、Provider 和策略组名称') }}</label>
      <p class="export-hint">{{ t('默认使用一致别名；不导出探测地址或原始错误。可选择状态仅代表快照时的检查结果。') }}</p>
      <fieldset class="export-fields"><legend>{{ t('结果字段') }}</legend>
        <div class="field-actions"><button @click="fields = exportFields.map(field => field.id)">{{ t('全选字段') }}</button><button @click="fields = []">{{ t('清空字段') }}</button></div>
        <div class="field-grid"><label v-for="field in exportFields" :key="field.id" class="field-choice"><input v-model="fields" type="checkbox" :value="field.id">{{ t(field.label) }}</label></div>
      </fieldset>
      <p class="export-hint">{{ t('服务、模式、测量边界和快照时间始终保留。CSV 使用固定列名，文件时间采用 ISO 8601。') }}</p>
      <div class="preview-heading"><label for="scan-export-preview">{{ t('内容预览') }}</label><span aria-live="polite">{{ t('{count} 条结果', {count}) }}</span></div>
      <textarea id="scan-export-preview" ref="preview" class="export-preview" :value="content" readonly spellcheck="false" :aria-label="t('内容预览')"/>
      <p v-if="!fields.length || !count" class="export-warning" role="status">{{ t(!fields.length ? '请至少选择一个结果字段。' : '此筛选下没有可导出的结果。') }}</p>
      <p v-if="message" :class="failed ? 'export-warning' : 'export-success'" role="status">{{ t(message) }}</p>
      <div class="export-footer"><button :disabled="!content || copying" @click="copy"><Copy :size="15" aria-hidden="true"/>{{ t(copying ? '正在复制' : '复制内容') }}</button><button class="primary" :disabled="!content" @click="download"><Download :size="15" aria-hidden="true"/>{{ t('下载文件') }}</button></div>
    </template>
  </dialog>
</template>

<style scoped>
.export-trigger, .export-footer button { display: inline-flex; align-items: center; justify-content: center; gap: 8px; }
.export-trigger { min-height: 40px; font-size: 13px; }
.result-export-dialog { box-sizing: border-box; width: min(760px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); padding: 24px; color: var(--text); background: var(--surface); border: 1px solid var(--line); border-radius: 12px; overflow-y: auto; }
.result-export-dialog::backdrop { background: var(--overlay); }
.export-heading, .preview-heading, .export-footer, .field-actions { display: flex; align-items: center; gap: 12px; }
.export-heading { justify-content: space-between; }
.export-heading h2 { font-size: 18px; margin: 0; }
.close-export { display: flex; padding: 10px; }
.export-description, .snapshot-time, .export-hint { font-size: 12px; color: var(--muted); line-height: 1.7; margin: 8px 0 12px; }
.export-options { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin: 20px 0 16px; }
.export-option { display: flex; flex-direction: column; gap: 8px; min-width: 0; font-size: 13px; }
.export-select { width: 100%; min-width: 0; min-height: 42px; padding: 9px; border: 1px solid var(--control-border); border-radius: var(--radius-sm); color: var(--text); background: var(--surface); }
.name-choice, .field-choice { display: flex; align-items: center; gap: 8px; font-size: 13px; line-height: 1.6; }
.name-choice input, .field-choice input { flex-shrink: 0; }
.export-fields { margin: 20px 0 12px; padding: 0; border: 0; min-width: 0; }
.export-fields legend { font-size: 13px; font-weight: 600; padding: 0; }
.field-actions { margin: 10px 0; }
.field-actions button { font-size: 12px; padding: 6px 10px; }
.field-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px 16px; }
.field-choice { min-height: 28px; }
.preview-heading { justify-content: space-between; font-size: 13px; margin: 16px 0 8px; }
.preview-heading span { color: var(--muted); }
.export-preview { display: block; box-sizing: border-box; width: 100%; min-height: 170px; resize: vertical; white-space: pre; font: 12px/1.6 var(--font-mono); border: 1px solid var(--control-border); border-radius: var(--radius-sm); padding: 12px; color: var(--text); background: var(--surface-muted); }
.export-footer { justify-content: flex-end; margin-top: 16px; }
.export-warning, .export-success { font-size: 13px; line-height: 1.6; margin: 10px 0; }
.export-warning { color: var(--warning); }.export-success { color: var(--green); }
@media(max-width:560px) { .result-export-dialog { padding: 16px; } .export-options { grid-template-columns: 1fr; gap: 12px; } .field-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; } .export-footer { flex-wrap: wrap; } .field-choice { min-height: 36px; } }
</style>
