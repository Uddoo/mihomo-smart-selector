<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {onMounted, ref} from 'vue'
import {api} from './api'
import type {RuntimeSettings} from './models'
const props = defineProps<{locked: boolean}>()
type Stats = {database_bytes:number; wal_bytes:number; scans:number; audit:number; unresolved:number}
const stats = ref<Stats | null>(null)
const policy = ref<RuntimeSettings | null>(null)
const dialog = ref<HTMLDialogElement | null>(null)
const busy = ref(false), error = ref(''), notice = ref('')
async function reload() {
  try { stats.value = await api<Stats>('/storage') }
  catch (e) { error.value = e instanceof Error ? e.message : '无法读取存储状态' }
}
async function preview() {
  if (props.locked || busy.value) return
  busy.value = true; error.value = ''
  try { policy.value = await api<RuntimeSettings>('/settings'); dialog.value?.showModal() }
  catch (e) { error.value = e instanceof Error ? e.message : '无法读取清理策略' }
  finally { busy.value = false }
}
async function cleanup() {
  if (props.locked || busy.value || !policy.value) return
  busy.value = true; error.value = ''
  try {
    const result = await api<{scans_deleted:number; audit_deleted:number; stats:Stats}>('/storage/cleanup', {method:'POST',body:JSON.stringify({confirm:true,revision:policy.value.revision})})
    stats.value = result.stats
    notice.value = `已清理 ${result.scans_deleted} 次扫描、${result.audit_deleted} 条审计记录。`
    dialog.value?.close()
  } catch (e) { error.value = e instanceof Error ? e.message : '清理失败'; dialog.value?.close() }
  finally { busy.value = false }
}
onMounted(reload)
</script>

<template>
  <section class="runtime-settings" aria-labelledby="storage-title">
    <div class="panel">
      <h2 id="storage-title">{{ t('历史存储') }}</h2>
      <p v-if="stats">{{ t('数据库 {p0} MB · WAL {p1} MB', {p0: (stats.database_bytes / 1048576).toFixed(2), p1: (stats.wal_bytes / 1048576).toFixed(2)}) }}<br>{{ t('{p0} 次扫描 · {p1} 条审计 · {p2} 条待核对', {p0: stats.scans, p1: stats.audit, p2: stats.unresolved}) }}</p>
      <p>{{ t('服务启动及每小时按已保存策略清理。运行中的扫描和待核对操作会保留；扫描运行时跳过清理。') }}</p>
      <p v-if="error" class="notice error" role="alert">{{ translateMessage(error) }}</p>
      <p v-if="notice" class="notice" role="status">{{ translateMessage(notice) }}</p>
      <div class="settings-actions">
        <button :disabled="locked || busy" @click="preview">{{ t('按保留策略清理') }}</button>
        <button :disabled="busy" @click="reload">{{ t('刷新存储状态') }}</button>
      </div>
      <dialog ref="dialog" class="choice-dialog" aria-labelledby="storage-cleanup-title">
        <h2 id="storage-cleanup-title">{{ t('确认清理历史') }}</h2>
        <p v-if="policy">{{ t('扫描保留 {p0} 天、最多 {p1} 次；审计保留 {p2} 天、最多 {p3} 条。超出任一限制的已结束记录将被删除，无法撤销。', {p0: policy.retention.scan_days, p1: policy.retention.max_scans, p2: policy.retention.audit_days, p3: policy.retention.max_audit}) }}</p>
        <p>{{ t('运行中任务、未确认操作及其关联扫描不参与清理。') }}</p>
        <div class="dialog-actions settings-actions">
          <button :disabled="busy" @click="dialog?.close()">{{ t('取消') }}</button>
          <button :disabled="busy || locked" @click="cleanup">{{ t('确认清理') }}</button>
        </div>
      </dialog>
    </div>
  </section>
</template>
