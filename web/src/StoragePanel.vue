<script setup lang="ts">
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
  <section class="panel">
    <h2>历史存储</h2>
    <p v-if="stats">数据库 {{ (stats.database_bytes / 1048576).toFixed(2) }} MB · WAL {{ (stats.wal_bytes / 1048576).toFixed(2) }} MB<br>{{ stats.scans }} 次扫描 · {{ stats.audit }} 条审计 · {{ stats.unresolved }} 条待核对</p>
    <p>服务启动及每小时按已保存策略清理。运行中的扫描和待核对操作会保留；扫描运行时跳过清理。</p>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <button :disabled="locked || busy" @click="preview">按保留策略清理</button>
    <button :disabled="busy" @click="reload">刷新存储状态</button>
    <dialog ref="dialog" class="choice-dialog">
      <h2>确认清理历史</h2>
      <p v-if="policy">扫描保留 {{ policy.retention.scan_days }} 天、最多 {{ policy.retention.max_scans }} 次；审计保留 {{ policy.retention.audit_days }} 天、最多 {{ policy.retention.max_audit }} 条。超出任一限制的已结束记录将被删除，无法撤销。</p>
      <p>运行中任务、未确认操作及其关联扫描不参与清理。</p>
      <button :disabled="busy" @click="dialog?.close()">取消</button><button :disabled="busy || locked" @click="cleanup">确认清理</button>
    </dialog>
  </section>
</template>
