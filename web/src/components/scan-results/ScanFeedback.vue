<script setup lang="ts">
import {computed} from 'vue'
import {CheckCircle2, CircleAlert, CirclePause, LoaderCircle, ScanLine} from '@lucide/vue'
import type {Scan} from '../../models'
import {t, translateMessage} from '../../i18n'

const props = defineProps<{
  scan: Scan | null
  starting: boolean
  startError: string
  stopPending: boolean
  stopRequested: 'batch' | 'now' | null
  connectionMode: 'live' | 'polling' | 'paused' | 'idle'
  demo: boolean
}>()
const emit = defineEmits<{stop: [afterCurrentBatch: boolean]}>()
const running = computed(() => props.scan?.status === 'running' && !props.starting)
const stopping = computed(() => props.stopRequested || (props.scan?.progress.stop_after_current_batch ? 'batch' : null))
const progress = computed(() => props.starting || props.startError ? null : props.scan?.progress)
const phase = computed(() => props.starting ? 'starting' : props.startError ? 'start-failed' : running.value && stopping.value ? 'stopping' : props.scan?.status || 'idle')
const tone = computed(() => ['failed', 'interrupted', 'start-failed'].includes(phase.value) ? 'failed' : phase.value === 'complete' ? 'complete' : ['cancelled', 'stopping'].includes(phase.value) ? 'stopped' : props.starting || running.value ? 'running' : 'idle')
const title = computed(() => {
  if (props.starting) return t('正在启动扫描')
  if (props.startError) return t('扫描未启动')
  if (running.value && stopping.value) return stopping.value === 'now' ? t('正在停止扫描') : t('将在本批结束后停止')
  if (running.value) return progress.value?.stage === 'refining' ? t('复测进行中') : t('初筛进行中')
  return translateMessage(({complete: '扫描完成', cancelled: '扫描已停止', interrupted: '扫描已中断', failed: '扫描失败'} as Record<string, string>)[props.scan?.status || ''] || '准备就绪')
})
const icon = computed(() => tone.value === 'failed' ? CircleAlert : tone.value === 'complete' ? CheckCircle2 : tone.value === 'stopped' ? CirclePause : props.starting || running.value ? LoaderCircle : ScanLine)
const percent = computed(() => progress.value?.total ? Math.max(0, Math.min(100, Math.round(progress.value.completed * 100 / progress.value.total))) : 0)
const context = computed(() => props.starting || props.startError ? t('确认新任务前保留上次结果。') : props.scan ? props.scan.request.target_group + ' · ' + translateMessage(props.scan.profile.label) : t('选择目标策略组和测试服务，开始扫描后在这里比较节点。'))
const nextStep = computed(() => {
  if (props.starting) return t('正在提交扫描请求，等待任务确认…')
  if (props.startError) return t('请检查错误提示后重新扫描，上次结果仍可查看。')
  if (running.value && stopping.value) return stopping.value === 'now' ? t('停止请求已提交，等待扫描确认；已返回的结果会保留。') : t('当前批次继续完成，之后不再启动新批次。也可立即停止。')
  if (running.value) return t('结果返回即更新排名，验证后分数仍可能变化')
  if (props.scan?.status === 'complete') return t('扫描不改变当前节点。请比较结果，再确认选择。')
  if (props.scan) return t('已返回的数据保留供查看；重新扫描完成后可选择节点。')
  return t('扫描不改变当前节点')
})
const transportLabel = computed(() => props.connectionMode === 'live' ? t('实时更新中') : props.connectionMode === 'paused' ? t('页面刷新已暂停，后台扫描继续运行') : t('定时刷新中'))
function duration(seconds?: number) {
  if (seconds == null || !Number.isFinite(seconds)) return '—'
  const whole = Math.max(0, Math.floor(seconds))
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, '0')}`
}
</script>

<template>
  <section class="scan-feedback" :class="tone" :aria-label="t('扫描状态')">
    <div class="scan-state" :class="tone">
      <div class="feedback-identity">
        <span class="feedback-icon" aria-hidden="true"><Transition name="scan-icon" mode="out-in"><component :is="icon" :key="phase" :size="20"/></Transition></span>
        <div class="feedback-copy"><b class="feedback-title" role="status" aria-live="polite" aria-atomic="true">{{ title }}</b><p class="feedback-context">{{ context }}</p></div>
      </div>
      <div class="feedback-tags"><span v-if="scan && !starting && !startError" class="feedback-count">{{ t('{p0} 个节点', {p0: scan.results?.length || 0}) }}</span><span v-if="demo" class="fixture-label">{{ t('示例数据') }}</span></div>
    </div>
    <div v-if="scan || starting || startError" class="feedback-body">
      <div class="feedback-progress-label"><span>{{ starting ? t('等待任务确认') : startError ? t('请重新尝试') : t('已完成探测任务 {p0} / {p1}', {p0: progress?.completed || 0, p1: progress?.total || 0}) }}<small v-if="progress?.total_batches"> · {{ t('第 {p0} / {p1} 批', {p0: progress.current_batch, p1: progress.total_batches}) }}</small></span><strong>{{ progress?.total ? percent + '%' : '—' }}</strong></div>
      <div class="progress-track" role="progressbar" :aria-label="t('扫描进度')" :aria-valuenow="progress?.total ? percent : undefined" :aria-valuemin="0" :aria-valuemax="100" :aria-valuetext="title + (progress?.total ? ' · ' + percent + '%' : '')"><span :style="{transform: `scaleX(${percent / 100})`}"></span></div>
      <dl class="feedback-metrics">
        <div><dt>{{ t('成功') }}</dt><dd>{{ progress ? progress.succeeded : '—' }}</dd></div>
        <div><dt>{{ t('失败') }}</dt><dd>{{ progress ? progress.failed : '—' }}</dd></div>
        <div><dt>{{ t('耗时') }}</dt><dd>{{ duration(progress?.elapsed_seconds) }}</dd></div>
        <div><dt>{{ t('预计剩余') }}</dt><dd>{{ running && !stopping && progress?.total && percent < 100 ? duration(progress.estimated_remaining_seconds) : '—' }}</dd></div>
      </dl>
    </div>
    <div v-if="scan || starting || startError" class="feedback-footer">
      <p class="feedback-next" role="status" aria-live="polite" aria-atomic="true">{{ stopPending ? t('正在提交停止请求…') : nextStep }}<small v-if="running">{{ transportLabel }}</small></p>
      <div v-if="running" class="feedback-actions">
        <button :disabled="stopPending || !!stopping" @click="emit('stop', true)">{{ stopping === 'batch' ? t('已请求批次结束后停止') : t('本批结束后停止') }}</button>
        <button class="danger" :disabled="stopPending || stopping === 'now'" @click="emit('stop', false)">{{ stopping === 'now' ? t('正在停止扫描') : t('立即停止') }}</button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.scan-feedback { --feedback-color:var(--muted);border-bottom:1px solid var(--line); }
.scan-feedback.running { --feedback-color:var(--info); }
.scan-feedback.complete { --feedback-color:var(--green); }
.scan-feedback.failed { --feedback-color:var(--red); }
.scan-feedback.stopped { --feedback-color:var(--warning); }
.scan-state { display:flex;justify-content:space-between;align-items:flex-start;gap:var(--space-4);padding:var(--space-4) 20px; }
.feedback-identity { display:flex;align-items:flex-start;gap:var(--space-2);min-width:0; }
.feedback-icon { display:grid;place-items:center;flex:0 0 20px;width:20px;height:24px;color:var(--feedback-color); }
.feedback-copy { min-width:0; }
.feedback-title { display:block;color:var(--feedback-color);font-size:14px;line-height:24px; }
.feedback-context { margin:var(--space-1) 0 0;color:var(--muted);font-size:12px;overflow-wrap:anywhere; }
.feedback-tags { display:flex;align-items:center;flex-wrap:wrap;justify-content:flex-end;gap:var(--space-2);flex-shrink:0; }
.feedback-count { font-size:12px;white-space:nowrap;font-variant-numeric:tabular-nums; }
.feedback-tags .fixture-label { margin:0; }
.feedback-body { padding:0 20px var(--space-4); }
.feedback-progress-label { display:flex;align-items:baseline;justify-content:space-between;gap:var(--space-3);font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums; }
.feedback-progress-label strong { color:var(--feedback-color);font-family:var(--font-mono);font-weight:500; }
.progress-track { height:4px;margin:var(--space-2) 0 var(--space-4);background:var(--soft);overflow:hidden;border-radius:var(--radius-sm); }
.progress-track span { display:block;width:100%;height:100%;background:var(--feedback-color);transform-origin:left center;transition:transform 180ms cubic-bezier(.23,1,.32,1); }
.feedback-metrics { display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:var(--space-4);margin:0; }
.feedback-metrics dt { font-size:12px;margin-bottom:var(--space-1); }
.feedback-metrics dd { font-family:var(--font-mono);font-size:14px;font-variant-numeric:tabular-nums; }
.feedback-footer { display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:var(--space-3);min-height:80px;padding:var(--space-3) 20px;border-top:1px solid var(--line); }
.feedback-next { flex:1 1 260px;min-width:0;margin:0;font-size:12px;color:var(--muted);line-height:1.7;overflow-wrap:anywhere; }
.feedback-next small { display:block;margin-top:var(--space-1);font-size:11px; }
.feedback-actions { display:flex;flex-wrap:wrap;gap:var(--space-2); }
.feedback-actions button { min-height:44px;font-size:12px; }
.scan-icon-enter-active { transition:opacity 160ms cubic-bezier(.23,1,.32,1),transform 160ms cubic-bezier(.23,1,.32,1); }
.scan-icon-leave-active { transition:opacity 80ms linear; }
.scan-icon-enter-from { opacity:0;transform:translateY(2px); }
.scan-icon-leave-to { opacity:0; }
@media(max-width:760px) {
  .scan-state { padding:var(--space-4);flex-wrap:wrap;gap:var(--space-2); }
  .feedback-tags { margin-left:28px; }
  .feedback-body { padding:0 var(--space-4) var(--space-4); }
  .feedback-footer { padding:var(--space-3) var(--space-4);min-height:124px;align-content:center; }
  .feedback-actions { width:100%; }.feedback-actions button { flex:1; }
  .feedback-metrics { gap:var(--space-2); }
}
@media(prefers-reduced-motion:reduce) {
  .progress-track span,.scan-icon-enter-active,.scan-icon-leave-active { transition:none; }
  .scan-icon-enter-from { transform:none; }
}
</style>
