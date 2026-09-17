<script setup lang="ts">
import {RefreshCw} from '@lucide/vue'
import {t, translateMessage, formatNumber} from './i18n'
import {useMonitorPlanEditor} from './useMonitorPlanEditor'
import MonitorCapacityNotice from './MonitorCapacityNotice.vue'
import type {MonitorPlanEditorProps} from './useMonitorPlanEditor'

const props = defineProps<MonitorPlanEditorProps>()
const emit = defineEmits<{saved: []; cancel: []; busy: [value: boolean]}>()
const {group, profile, chosen, candidateLimit, query, catalog, catalogBusy, catalogError, retention, retentionError, loadRetention, saveError, saving, profiles, filtered, missing, maxLimit, validLimit, overLimit, validProfile, estimated, budget, canSave, loadCatalog, save} = useMonitorPlanEditor(props, {saved: () => emit('saved'), busy: value => emit('busy', value)})
</script>

<template>
  <form class="monitor-config" @submit.prevent="save">
    <h3>{{ plan ? t('调整监控方案') : t('创建第一个监控方案') }}</h3>
    <p>{{ t('一次启用，后台持续运行。默认加入当前叶子节点及最近扫描候选。') }}</p>
    <div class="monitor-fields">
      <label>{{ t('监控策略组') }}<select v-model="group" :disabled="disabled"><option value="" disabled>{{ t('选择策略组') }}</option><option v-for="g in groups" :key="g.name" :value="g.name">{{ g.name }}</option></select></label>
      <label>{{ t('监控服务模板') }}<select v-model="profile" :disabled="disabled"><option value="" disabled>{{ t('选择服务') }}</option><option v-for="p in profiles" :key="p.id" :value="p.id">{{ translateMessage(p.label) }}</option></select></label>
    </div>
    <div class="monitor-limit">
      <label for="monitor-candidate-limit">{{ t('监控候选上限') }}<input id="monitor-candidate-limit" v-model.number="candidateLimit" type="number" name="monitor-candidate-limit" inputmode="numeric" min="1" :max="maxLimit" step="1" required :disabled="disabled" :aria-invalid="!validLimit || overLimit" aria-describedby="monitor-limit-hint monitor-limit-error"></label>
      <p id="monitor-limit-hint">{{ t('可设为 1–{p0} 个，默认 6 个。提高上限后，手动勾选需要监控的节点。', {p0: maxLimit}) }}</p>
    </div>
    <p id="monitor-limit-error" class="bad" :role="!validLimit || overLimit ? 'alert' : undefined"><template v-if="!validLimit">{{ t('请输入 1–{p0} 的整数。', {p0: maxLimit}) }}</template><template v-else-if="overLimit">{{ t('已选 {p0} 个，超过上限 {p1} 个。请取消部分选择或提高上限；已选节点不会自动移除。', {p0: chosen.length, p1: candidateLimit}) }}</template></p>
    <p v-if="catalogError" class="bad" role="alert">{{ translateMessage(catalogError) }}</p>
    <p v-if="catalog && !validProfile" class="bad" role="alert">{{ t('监控模板须包含 1–6 个探测目标，请调整服务模板') }}</p>
    <div class="monitor-picker-head"><label>{{ t('搜索监控候选') }}<input v-model="query" type="search" name="monitor-candidate-query" autocomplete="off" :spellcheck="false" :placeholder="t('输入节点名称…')"></label><span role="status">{{ t('{p0} / {p1} 已选', {p0: chosen.length, p1: validLimit ? candidateLimit : '—'}) }}</span><button type="button" :disabled="disabled || catalogBusy" @click="loadCatalog(true)"><RefreshCw :size="14" aria-hidden="true"/>{{ t('刷新候选') }}</button></div>
    <p v-if="catalogBusy" role="status">{{ t('正在核实策略组与候选身份…') }}</p>
    <fieldset class="monitor-picker" :disabled="disabled || catalogBusy || !catalog"><legend class="sr-only">{{ t('选择监控节点') }}</legend>
      <label v-for="n in filtered" :key="n.id"><input v-model="chosen" type="checkbox" :value="n.name" :disabled="!chosen.includes(n.name) && (!validLimit || chosen.length >= Number(candidateLimit))"><span>{{ n.name }} <small v-if="catalog?.current === n.name">{{ t('当前选择') }}</small><small>{{ n.provider || t('独立节点') }} · {{ n.protocol }}</small></span></label>
    </fieldset>
    <div v-if="missing.length" class="bad" role="alert"><p>{{ t('部分已选节点不再可用，请刷新候选或移除后保存。') }}</p><button v-for="name in missing" :key="name" type="button" :disabled="disabled" @click="chosen = chosen.filter(n => n !== name)">{{ t('移除 {p0}', {p0: name}) }}</button></div>
    <p v-if="catalog && !filtered.length">{{ t('没有匹配的候选；请调整搜索或检查策略组。') }}</p>
    <p v-if="catalog && validProfile" class="monitor-note">{{ t('每个节点探测 {p0} 个目标。基准采样每 2 分钟，当前节点附加检查每 30 秒；预计约 {p1} 次探测/天，确认请求另计。后台共享预算为每分钟 {p2} 次。', {p0: catalog.probe_count, p1: formatNumber(estimated), p2: budget}) }}</p>
    <p class="monitor-note">{{ t('节点越多，探测与存储开销越高。慢响应或扫描占用可能造成缺测；缺测不会计为节点失败。') }}</p>
    <MonitorCapacityNotice v-if="retention" :candidate-count="chosen.length" :policy="retention"/>
    <p v-if="retentionError && !retention" class="monitor-note" role="status">{{ t('保留策略读取失败，暂不能估算容量。') }} <button type="button" @click="loadRetention">{{ t('重新读取') }}</button></p>
    <p v-if="plan" class="monitor-note">{{ t('暂停、继续同一方案保留评分。调整候选列表会保留未变化节点的历史；模板或节点身份改变时分开记录，可在历史序列中查看。') }}</p>
    <p v-if="saveError" class="bad" role="alert">{{ translateMessage(saveError) }}</p>
    <div class="monitor-actions"><button class="primary" :disabled="!canSave">{{ saving ? t('正在保存') : plan ? t('保存并监控') : t('开始监控') }}</button><button v-if="plan" type="button" :disabled="disabled" @click="emit('cancel')">{{ t('取消调整') }}</button></div>
  </form>
</template>

<style scoped>
.monitor-config {border:1px solid var(--line);border-radius:var(--radius-md);background:var(--surface);padding:var(--monitor-panel-padding,var(--space-5));min-width:0;}
h3 {font-size:16px;margin:0 0 9px;}
p {color:var(--muted);font-size:13px;line-height:1.7;margin:8px 0;overflow-wrap:anywhere;}
.monitor-fields {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:var(--space-4);}
label {font-size:13px;}
.monitor-fields label,.monitor-picker-head label,.monitor-limit label {display:grid;gap:8px;min-width:0;}
select,input:not([type=checkbox]) {border:1px solid var(--control-border);border-radius:var(--radius-sm);background:var(--surface);color:var(--text);padding:10px;min-height:44px;min-width:0;width:100%;}
.monitor-limit {display:flex;align-items:flex-end;gap:18px;flex-wrap:wrap;margin-top:18px;}
.monitor-limit label {width:140px;}.monitor-limit p {flex:1;min-width:180px;max-width:65ch;}
.monitor-limit input[aria-invalid=true] {border-color:var(--red);}
.monitor-picker-head,.monitor-actions {display:flex;align-items:center;gap:12px;flex-wrap:wrap;}
.monitor-picker-head {align-items:flex-end;margin:var(--space-4) 0 var(--space-3);}.monitor-picker-head label{flex:1;min-width:160px;}
.monitor-picker-head span {display:flex;align-items:center;min-height:44px;font-size:13px;font-variant-numeric:tabular-nums;}
button {display:inline-flex;align-items:center;gap:var(--space-2);min-height:44px;}
.monitor-picker {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:9px;max-height:320px;overflow:auto;border:1px solid var(--line);padding:12px;border-radius:var(--radius-sm);min-width:0;margin:0;}
.monitor-picker label {display:flex;align-items:flex-start;gap:8px;overflow-wrap:anywhere;padding:6px;min-height:44px;}
.monitor-picker input {margin-top:3px;flex-shrink:0;}.monitor-picker small {display:block;margin-top:5px;}
.monitor-note {font-size:12px;}.bad {color:var(--red)!important;}.monitor-actions {margin-top:16px;}
@media(max-width:760px){select,input:not([type=checkbox]){font-size:16px;}}
@media(max-width:640px){.monitor-fields,.monitor-picker{grid-template-columns:1fr;}.monitor-picker-head label{flex-basis:100%}.monitor-picker-head>button{margin-left:auto}.monitor-limit{gap:4px;}.monitor-limit p{flex-basis:100%;}}
</style>
