<script setup lang="ts">
import {t, translateMessage, formatDate} from './i18n'
import {computed} from 'vue'
import {usePagination} from './usePagination'
import ListPagination from './ListPagination.vue'
import TableSearch from './TableSearch.vue'
import type {Workbench} from './useWorkbench'
import {RefreshCw, ChevronDown, CheckCircle2, Radio, CircleAlert, CirclePause, ScanLine} from '@lucide/vue'
const {state} = defineProps<{state: Workbench}>()
const { syncError, refreshScan, connectionMode, health, groups, providers, availableRegions, group, serviceID, services, loading, discoveryValid, areas, providerSet, mode, preview, failure, switching, starting, showProfile, current, results, candidate, scanLabel, configLocked, progress, percent, profile, groupMissing, binding, invalidBindings, serviceSource, profileReady, openRecent, load, saveBinding, provider, start, stop, selectionReason, choose, regionLabel, percentLabel, latency, clock, statusLabel, statusTone, probeKind, scan, running, recent } = state
import {ref, nextTick, onMounted, onBeforeUnmount, useTemplateRef, watch} from 'vue'
import CandidateDetails from './CandidateDetails.vue'
import ScanCompatibility from './ScanCompatibility.vue'
import ScanRegionFilter from './ScanRegionFilter.vue'
const resultQuery = ref('')
const filteredResults = computed(() => results.value.filter(row => !resultQuery.value || row.name.toLowerCase().includes(resultQuery.value.toLowerCase())))
const {page: resultPage, pageCount, visible: pageResults, locate} = usePagination(filteredResults)
watch([resultQuery, () => state.scan.value?.id], () => { resultPage.value = 1 }, {flush: 'sync'})
function locateNode(name?: string) { resultQuery.value = ''; locate(results.value.findIndex(row => row.name === name)) }
const mobileQuery = window.matchMedia('(max-width: 760px)')
const mobile = ref(mobileQuery.matches)
const configExpanded = ref(false)
const configPanel = useTemplateRef<HTMLElement>('configPanel')
const configToggle = useTemplateRef<HTMLButtonElement>('configToggle')
const compactConfig = computed(() => mobile.value && !!scan.value && !running.value)
watch(compactConfig, async compact => {
  if (compact && !configExpanded.value && configPanel.value?.contains(document.activeElement)) {
    await nextTick()
    configToggle.value?.focus()
  }
})
const drawerOpen = ref(false)
const scanFailed = computed(() => scan.value?.status === 'failed' || scan.value?.status === 'interrupted')
const scanTone = computed(() => scanFailed.value ? 'failed' : scan.value?.status === 'complete' ? 'complete' : running.value ? 'running' : 'idle')
const emptyTitle = computed(() => results.value.length ? '没有匹配的节点' : running.value ? '正在检测节点' : scanFailed.value ? '本次扫描未完成' : scan.value ? '本次扫描没有结果' : '准备开始扫描')
const emptyDescription = computed(() => results.value.length ? '换一个节点名称，或清空搜索查看全部结果。' : running.value ? '第一个节点完成后，检测结果会实时显示在这里。' : scan.value ? '检查上方扫描状态与配置，然后重新扫描。' : '选择目标策略组和测试服务，开始扫描后在这里比较节点。')
function scanStatus(value: string) { return ({running: '进行中', complete: '已完成', cancelled: '已停止', failed: '失败', interrupted: '已中断'} as Record<string, string>)[value] || value }
const detailDialog = ref<HTMLDialogElement | null>(null)
function closeDetails() { detailDialog.value?.close(); drawerOpen.value = false }
function openDetails(name: string) {
  state.focusedName.value = name
  if (mobile.value) { drawerOpen.value = true; detailDialog.value?.showModal() }
}
function resize() { closeDetails(); mobile.value = mobileQuery.matches }
watch(() => state.scan.value?.id, closeDetails)
onMounted(() => mobileQuery.addEventListener('change', resize))
onBeforeUnmount(() => { closeDetails(); mobileQuery.removeEventListener('change', resize) })
</script>

<template>
      <section class="scan-workbench">
        <p v-if="syncError" class="notice warning" role="alert">{{ t('{p0}。保留上次结果，等待恢复。', {p0: translateMessage(syncError)}) }}<button @click="refreshScan">{{ t('重新读取扫描') }}</button></p>
        <label v-if="recent.length" class="profile-scope recent-scan"><span>{{ t('活动 / 最近扫描') }}</span><select :value="scan?.id" :disabled="starting || switching" @change="openRecent"><option v-for="item in recent" :key="item.id" :value="item.id">{{ item.request.target_group }} · {{ t(scanStatus(item.status)) }} · {{ formatDate(new Date(item.started_at)) }}</option></select></label>
        <div class="workbench-status"><span>{{ t('当前节点') }} <b>{{ current?.now || t('等待 Controller 回读') }}</b></span><span :class="health?.mihomo_connected ? 'good' : 'bad'">{{ health?.mihomo_connected ? t('Controller 已连接') : t('Controller 不可用') }}</span></div>
        <div v-if="compactConfig" class="scan-quick-config">
          <button ref="configToggle" class="scan-config-toggle" :aria-expanded="configExpanded" aria-controls="scan-configuration" @click="configExpanded = !configExpanded"><span>{{ t('扫描设置') }}<small>{{ translateMessage(profile?.label || t('等待评分配置')) }}</small></span><ChevronDown :size="16" :class="{expanded: configExpanded}" aria-hidden="true"/></button>
          <button v-if="!configExpanded" class="scan-button primary" :disabled="configLocked || !group || !profileReady" @click="start"><RefreshCw :size="16" aria-hidden="true"/>{{ starting ? t('正在启动') : t('重新扫描') }}</button>
        </div>
        <div v-show="!compactConfig || configExpanded" id="scan-configuration" ref="configPanel" class="scan-config">
        <fieldset class="command" :disabled="configLocked || loading">
          <legend class="sr-only">{{ t('扫描配置') }}</legend>
          <label>{{ t('目标策略组') }}<select v-model="group" :aria-label="t('目标策略组')"><option v-if="groupMissing" :value="group">{{ t('{p0}（已失效）', {p0: group}) }}</option><option v-for="item in groups" :key="item.name" :value="item.name">{{ item.name }}</option></select></label>
          <label>{{ t('测试服务') }}<select v-model="serviceID" :aria-label="t('测试服务')"><option value="">{{ t('自动使用绑定或推荐') }}</option><option v-for="item in services?.profiles || []" :key="item.id" :value="item.id">{{ translateMessage(item.label) }}{{ item.requires_configuration ? t('（待配置）') : '' }}</option></select></label>
          <button class="scan-button primary" :disabled="configLocked || !group || !profileReady" @click="start"><RefreshCw :size="16" :class="{spinning: running}" aria-hidden="true"/>{{ starting ? t('正在启动') : running ? t('扫描中') : scan ? t('重新扫描') : t('开始扫描') }}</button>
        </fieldset>
        <details class="advanced-config">
          <summary><span>{{ t('高级筛选与服务绑定') }}</span><small>{{ areas.length ? areas.map(regionLabel).join(' / ') : t('全部地区') }} · {{ providerSet.length ? providerSet.length + t(' 个 Provider') : t('全部 Provider') }} · {{ mode === 'stable' ? t('稳定模式') : t('快速模式') }}</small><ChevronDown :size="16"/></summary>
          <fieldset class="command advanced-fields" :disabled="configLocked || loading"><legend class="sr-only">{{ t('高级扫描配置') }}</legend>
          <ScanRegionFilter v-model="areas" :regions="availableRegions" :region-label="regionLabel"/>
          <details class="providers">
            <summary>Provider <span>{{ providerSet.length || t('全部') }}</span><ChevronDown :size="14"/></summary>
            <div class="provider-options"><label v-for="item in providers" :key="item.name"><input type="checkbox" :checked="providerSet.includes(item.name)" @change="provider(item.name)">{{ item.name }}</label></div>
          </details>
          <label>{{ t('模式') }}<select v-model="mode" :aria-label="t('模式')"><option value="quick">{{ t('快速') }}</option><option value="stable">{{ t('稳定') }}</option></select></label>
          </fieldset>
        <div class="service-binding">
          <span>{{ translateMessage(serviceSource) }}</span>
          <button :disabled="configLocked || loading || !profile || groupMissing" @click="saveBinding()">{{ t('保存绑定') }}</button>
          <button v-if="binding" :disabled="configLocked || loading" @click="saveBinding(group, true)">{{ t('移除绑定') }}</button>
          <button :disabled="configLocked || loading" @click="load()">{{ loading ? t('正在刷新') : t('刷新策略组') }}</button>
          <small>{{ t('每 30 秒刷新 · 扫描期间暂停刷新') }}</small>
        </div>
        </details>
        <p v-if="groupMissing" class="preflight warning" role="alert">{{ t('目标策略组已删除或改名，请重新选择策略组；不会自动迁移绑定。') }}</p>
        <div v-for="item in invalidBindings" :key="item.group" class="service-binding invalid-binding" role="status">
          <span>{{ t('失效绑定：{p0} → {p1}（{p2}）', {p0: item.group, p1: item.profile_id, p2: item.status === 'group_missing' ? t('策略组不存在或不再是 Selector') : t('服务模板不存在')}) }}</span>
          <button :disabled="configLocked || loading" @click="saveBinding(item.group, true)">{{ t('移除此失效绑定') }}</button>
        </div>
        <div class="config-summary"><span><b>{{ translateMessage(profile?.label || t('等待评分配置')) }}</b><span v-if="preview?.ready"> {{ t('· {p0} 个候选 · {p1} 批', {p0: preview.candidate_count, p1: preview.batch_count}) }}</span></span><button :aria-expanded="showProfile" aria-controls="probe-profile" @click="showProfile = !showProfile">{{ t('探测配置与地址') }}<ChevronDown :size="15" :class="{expanded: showProfile}"/></button></div>

        <section v-if="profile && (showProfile || profile.requires_configuration)" id="probe-profile" class="profile-card" :class="{warning: profile.requires_configuration}">
          <div class="profile-copy">
            <small>{{ t('本次评分配置 · {p0}', {p0: profile.id}) }}</small>
            <h2>{{ translateMessage(profile.label) }}</h2>
            <p>{{ translateMessage(profile.description) }}</p>
          </div>
          <dl>
            <div><dt>{{ t('可达性') }}</dt><dd>{{ t('{p0} 个 HTTPS 探测', {p0: profile.probe_count}) }}</dd></div>
            <div><dt>{{ t('严格验证') }}</dt><dd>{{ profile.strict_probe_count ? (profile.strict_verification_available ? profile.strict_probe_count + t(' 项可用') : profile.strict_probe_count + t(' 项待配置')) : t('未请求') }}</dd></div>
            <div><dt>{{ t('地区验证') }}</dt><dd>{{ profile.expected_regions?.length ? profile.expected_regions.join(' / ') : t('未配置预期出口') }}</dd></div>
            <div><dt>{{ t('传输范围') }}</dt><dd>{{ translateMessage(profile.transport_scope) }}</dd></div>
          </dl>
          <section v-if="profile.targets?.length" class="profile-targets" :aria-label="t('测试目标地址')">
            <div class="target-heading"><b>{{ t('测试目标地址') }}</b><small>{{ t('公开内置探测地址') }}</small></div>
            <ul>
              <li v-for="target in profile.targets" :key="target.kind + target.name">
                <div><b>{{ translateMessage(probeKind(target.kind)) }} · {{ target.name }}</b><small>{{ t('期望 HTTP {p0}', {p0: target.expected_status}) }}</small></div>
                <code v-if="target.address_visible">{{ target.address }}</code>
                <span v-else class="private-target">{{ t('私有目标已配置，不向 LAN 浏览器展示') }}</span>
              </li>
            </ul>
          </section>
          <p v-if="profile.requires_configuration" class="profile-hint">{{ translateMessage(profile.setup_hint) }}</p>
          <p v-if="profile.require_strict || profile.require_region" class="profile-scope">{{ t('切换条件：{p0}{p1}', {p0: profile.require_strict ? t('严格验证通过；') : '', p1: profile.require_region ? t('出口地区匹配；') : ''}) }}</p>
          <p v-if="!profile.requires_configuration" class="profile-scope">{{ t('评分衡量此服务的可达性、时延与稳定性；除非“严格验证”已通过，否则不把结果视为登录、解锁或播放证明。') }}</p>
        </section>
        </div>

        <ScanCompatibility v-if="!running && !groupMissing && discoveryValid" :preview="preview" :disabled="configLocked || loading" :navigation="state.nestedNavigation.value" @select-group="state.selectNestedGroup" @return-to-parent="state.returnToParentGroup" @clear-filters="state.clearScanFilters"/>
        <div v-if="!running && !groupMissing && discoveryValid && !profile?.requires_configuration && !preview" class="preflight warning"><b>{{ t('扫描预检') }}</b><span>{{ translateMessage(!group ? t('未发现可选择的 Selector 策略组。') : failure || t('正在加载预检。')) }}</span></div>

        <div v-if="running" class="progress">
          <div><b>{{ t('{p0} · 已完成探测任务 {p1} / {p2} · 第 {p3} / {p4} 批', {p0: progress?.stage === 'refining' ? t('复测阶段') : t('初筛阶段'), p1: progress?.completed || 0, p2: progress?.total || 0, p3: progress?.current_batch || 0, p4: progress?.total_batches || 0}) }}</b><strong>{{ percent }}%</strong></div>
          <div class="progress-track" role="progressbar" :aria-label="t('扫描进度')" :aria-valuenow="percent" :aria-valuemin="0" :aria-valuemax="100"><span :style="{transform: `scaleX(${percent / 100})`}"></span></div>
          <section><span>{{ t('成功') }} <b>{{ progress?.succeeded || 0 }}</b></span><span>{{ t('失败') }} <b>{{ progress?.failed || 0 }}</b></span><span>{{ t('耗时') }} <b>{{ clock(progress?.elapsed_seconds) }}</b></span><span>{{ t('预计剩余') }} <b>{{ clock(progress?.estimated_remaining_seconds) }}</b></span></section>
          <p><button @click="stop(true)">{{ t('本批结束后停止') }}</button><button class="danger" @click="stop(false)">{{ t('立即停止') }}</button></p>
        </div>

        <div class="result-workspace">
        <div v-if="running" class="scope-note" role="status">{{ connectionMode === 'live' ? t('实时更新中') : connectionMode === 'paused' ? t('页面刷新已暂停，后台扫描继续运行') : t('定时刷新中') }}</div>
        <div class="scan-state" :class="scanTone" role="status"><span><CircleAlert v-if="scanFailed" :size="18" aria-hidden="true"/><CheckCircle2 v-else-if="scan?.status === 'complete'" :size="18" aria-hidden="true"/><CirclePause v-else-if="scan?.status === 'cancelled'" :size="18" aria-hidden="true"/><Radio v-else :size="18" aria-hidden="true"/><b>{{ translateMessage(scanLabel) }}</b> {{ t('· {p0} 个节点', {p0: results.length}) }}</span><small>{{ scan ? scan.request.target_group + ' · ' + t(scan.profile.label) + ' · ' : '' }}{{ running ? t('结果返回即更新排名，验证后分数仍可能变化') : t('扫描不改变当前节点') }}</small><span v-if="health?.mihomo_version === 'dev-mock'" class="fixture-label">{{ t('示例数据') }}</span></div>
        <p v-if="scanFailed || scan?.status === 'cancelled'" class="scan-recovery">{{ t('{p0}，已返回的数据保留供查看；重新扫描完成后可选择节点。', {p0: scanFailed ? t('本次扫描未完成') : t('本次扫描已停止')}) }}</p>
        <div class="grid">
          <section class="ranking panel">
            <div class="head">
              <div><h2>{{ t('实时排名') }}</h2><p>{{ scan?.status === 'complete' ? t('复测节点优先 · 性能满分 90 · 仅可选择符合条件的节点') : t('结果返回即排序 · 扫描完成后可选择') }}</p></div>
            </div>
            <div class="ranking-tools"><TableSearch id="scan-result-query" v-model="resultQuery" :label="t('搜索扫描结果')" :placeholder="t('搜索扫描结果…')" :disabled="!results.length"/><button :disabled="!results.some(row => row.name === current?.now)" @click="locateNode(current?.now)">{{ t('定位当前节点') }}</button><button :disabled="!candidate" @click="locateNode(candidate?.name)">{{ t('定位候选') }}</button></div>
            <div v-if="!pageResults.length" class="ranking-empty" role="status">
              <ScanLine :size="30" :stroke-width="1.5" aria-hidden="true"/>
              <h3>{{ translateMessage(emptyTitle) }}</h3><p>{{ translateMessage(emptyDescription) }}</p>
              <button v-if="resultQuery" @click="resultQuery = ''">{{ t('清空搜索') }}</button>
            </div>
            <div v-if="!mobile && pageResults.length" class="scroll desktop-ranking" tabindex="0" role="region" :aria-label="t('节点实时排名列表')">
              <table :aria-label="t('节点实时排名')">
                <thead><tr><th scope="col">{{ t('排名') }}</th><th scope="col">{{ t('节点 / Provider') }}</th><th scope="col">{{ t('地区') }}</th><th scope="col" class="numeric">{{ t('成功率') }}</th><th scope="col" class="numeric">P95</th><th scope="col" class="numeric">{{ t('性能评分') }}</th><th scope="col" class="action-column">{{ t('操作') }}</th></tr></thead>
                <tbody>
                  <tr v-for="result in pageResults" :key="result.name" :class="{selected: candidate?.name === result.name}" @click="openDetails(result.name)">
                    <td class="rank">{{ result.rank }}</td>
                    <td>
                      <div class="node-name"><button class="node-focus" :aria-label="t('查看 {node} 详情', {node: result.name})" :aria-pressed="candidate?.name === result.name" @click.stop="openDetails(result.name)">{{ result.name }}</button><span v-if="current?.now === result.name" class="current-tag">{{ t('当前') }}</span></div><small>{{ result.provider }}</small>
                      <div class="state-row"><span class="state">{{ result.stage === 'refined' ? t('已复测') : t('仅初筛') }}</span>
                        <span :class="['state', statusTone(result.reachability_status)]">{{ translateMessage(statusLabel(result.reachability_status)) }}</span>
                        <span v-if="statusTone(result.restriction_status) === 'bad'" class="state bad">{{ translateMessage(statusLabel(result.restriction_status)) }}</span>
                        <span v-if="statusTone(result.strict_verification_status) === 'bad'" class="state bad">{{ t('严格 · {p0}', {p0: translateMessage(statusLabel(result.strict_verification_status))}) }}</span>
                        <span v-if="statusTone(result.region_verification_status) === 'bad'" class="state bad">{{ translateMessage(statusLabel(result.region_verification_status)) }}</span>
                      </div>
                    </td>
                    <td>{{ regionLabel(result.inferred_region) }}</td>
                    <td class="numeric">{{ percentLabel(result.success_rate) }}</td>
                    <td class="numeric">{{ latency(result.p95_ms) }}</td>
                    <td class="score numeric"><b>{{ result.score.toFixed(1) }}</b></td>
                    <td class="action-column"><button class="row-select" :aria-label="t('选择 ') + result.name" :title="translateMessage(selectionReason(result) || t('确认切换到 ') + result.name)" :disabled="!!selectionReason(result)" @click.stop="choose(result)">{{ t('选择') }}</button></td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else-if="mobile && pageResults.length" class="mobile-ranking">
              <button v-for="result in pageResults" :key="result.name" class="node-card" :class="{selected: candidate?.name === result.name}" :aria-label="t('查看 {node} 详情', {node: result.name})" @click="openDetails(result.name)">
                <span class="node-card-title"><strong>{{ result.rank }}. {{ result.name }}</strong><span v-if="current?.now === result.name" class="current-tag">{{ t('当前') }}</span><span class="score">{{ result.score.toFixed(1) }}<small>/90</small></span></span>
                <span class="node-card-meta">{{ result.provider || t('未知 Provider') }} · {{ regionLabel(result.inferred_region) }}</span>
                <span class="node-card-metrics"><span>{{ t('成功率') }} <b>{{ percentLabel(result.success_rate) }}</b></span><span>P95 <b>{{ latency(result.p95_ms) }}</b></span><span>{{ result.stage === 'refined' ? t('已复测') : t('仅初筛') }}</span></span>
                <span class="node-card-reason">{{ translateMessage(selectionReason(result) || t('可确认切换 · 查看证据与详情')) }}</span>
              </button>
            </div>
            <ListPagination :page="resultPage" :pages="pageCount" :total="filteredResults.length" @change="resultPage = $event"/>
          </section>

          <CandidateDetails v-if="!mobile" :state="state"/>
          <dialog v-else ref="detailDialog" class="candidate-drawer" :aria-label="t('候选详情')" @close="drawerOpen = false">
            <div class="drawer-heading"><b>{{ t('候选详情') }}</b><span v-if="health?.mihomo_version === 'dev-mock'" class="fixture-label">{{ t('示例数据') }}</span><button :aria-label="t('关闭候选详情')" @click="closeDetails">{{ t('收起详情') }}</button></div>
            <CandidateDetails :state="state"/>
          </dialog>
        </div>
        </div>
        <p v-if="preview && !running" class="scope-note">{{ t('预计最多 {p0} 次延迟请求；初筛 {p1} 个节点，复测最多 {p2} 个（含当前节点预留名额）。', {p0: preview.probe_requests, p1: preview.candidate_count, p2: preview.refine_candidates}) }}</p>
        <p class="scope-note">{{ t('评分反映可达性、时延与稳定性，不代表登录、解锁或播放验证通过。') }}</p>
      </section>
</template>
