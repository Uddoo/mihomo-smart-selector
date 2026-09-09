<script setup lang="ts">
import {computed} from 'vue'
import {usePagination} from './usePagination'
import ListPagination from './ListPagination.vue'
import type {Workbench} from './useWorkbench'
import {RefreshCw, ChevronDown, CheckCircle2, Radio} from '@lucide/vue'
const {state} = defineProps<{state: Workbench}>()
const { syncError, refreshScan, connectionMode, health, groups, providers, regions, group, serviceID, services, loading, discoveryValid, areas, providerSet, mode, preview, failure, switching, starting, showProfile, current, results, candidate, scanLabel, configLocked, progress, percent, profile, groupMissing, binding, invalidBindings, serviceSource, profileReady, openRecent, load, saveBinding, area, provider, start, stop, selectionReason, choose, regionLabel, percentLabel, latency, clock, statusLabel, statusTone, probeKind, scan, running, recent } = state
import {ref, onMounted, onBeforeUnmount, watch} from 'vue'
import CandidateDetails from './CandidateDetails.vue'
const resultQuery = ref('')
const filteredResults = computed(() => results.value.filter(row => !resultQuery.value || row.name.toLowerCase().includes(resultQuery.value.toLowerCase())))
const {page: resultPage, pageCount, visible: pageResults, locate} = usePagination(filteredResults)
watch([resultQuery, () => state.scan.value?.id], () => { resultPage.value = 1 }, {flush: 'sync'})
function locateNode(name?: string) { resultQuery.value = ''; locate(results.value.findIndex(row => row.name === name)) }
const mobileQuery = window.matchMedia('(max-width: 760px)')
const mobile = ref(mobileQuery.matches)
const drawerOpen = ref(false)
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
      <section>
        <p v-if="syncError" class="notice warning" role="alert">{{ syncError }}。保留上次结果，等待恢复。<button @click="refreshScan">重新读取扫描</button></p>
        <label v-if="recent.length" class="profile-scope">活动 / 最近扫描<select :value="scan?.id" :disabled="starting || switching" @change="openRecent"><option v-for="item in recent" :key="item.id" :value="item.id">{{ item.request.target_group }} · {{ scanStatus(item.status) }} · {{ new Date(item.started_at).toLocaleString() }}</option></select></label>
        <div class="workbench-status"><span>当前节点 <b>{{ current?.now || '等待 Controller 回读' }}</b></span><span :class="health?.mihomo_connected ? 'good' : 'bad'">{{ health?.mihomo_connected ? 'Controller 已连接' : 'Controller 不可用' }}</span></div>
        <div class="scan-config">
        <fieldset class="command" :disabled="configLocked || loading">
          <legend class="sr-only">扫描配置</legend>
          <label>目标策略组<select v-model="group" aria-label="目标策略组"><option v-if="groupMissing" :value="group">{{ group }}（已失效）</option><option v-for="item in groups" :key="item.name" :value="item.name">{{ item.name }}</option></select></label>
          <label>测试服务<select v-model="serviceID" aria-label="测试服务"><option value="">自动使用绑定或推荐</option><option v-for="item in services?.profiles || []" :key="item.id" :value="item.id">{{ item.label }}{{ item.requires_configuration ? '（待配置）' : '' }}</option></select></label>
          <button :class="['scan-button', {primary: !scan}]" :disabled="configLocked || !group || !profileReady" @click="start"><RefreshCw :size="16" :class="{spinning: running}"/>{{ starting ? '正在启动' : running ? '扫描中' : scan ? '重新扫描' : '开始扫描' }}</button>
        </fieldset>
        <details class="advanced-config">
          <summary><span>高级筛选与服务绑定</span><small>{{ areas.length ? areas.map(regionLabel).join(' / ') : '全部地区' }} · {{ providerSet.length ? providerSet.length + ' 个 Provider' : '全部 Provider' }} · {{ mode === 'stable' ? '稳定模式' : '快速模式' }}</small><ChevronDown :size="16"/></summary>
          <fieldset class="command advanced-fields" :disabled="configLocked || loading"><legend class="sr-only">高级扫描配置</legend>
          <div class="chips">
            <b>地区</b>
            <button :class="{active: !areas.length}" :aria-pressed="!areas.length" @click="areas = []">全部</button>
            <button v-for="item in regions" :key="item.code" :class="{active: areas.includes(item.code)}" :aria-pressed="areas.includes(item.code)" @click="area(item.code)">{{ regionLabel(item.code) }}</button>
          </div>
          <details class="providers">
            <summary>Provider <span>{{ providerSet.length || '全部' }}</span><ChevronDown :size="14"/></summary>
            <div class="provider-options"><label v-for="item in providers" :key="item.name"><input type="checkbox" :checked="providerSet.includes(item.name)" @change="provider(item.name)">{{ item.name }}</label></div>
          </details>
          <label>模式<select v-model="mode" aria-label="模式"><option value="quick">快速</option><option value="stable">稳定</option></select></label>
          </fieldset>
        <div class="service-binding">
          <span>{{ serviceSource }}</span>
          <button :disabled="configLocked || loading || !profile || groupMissing" @click="saveBinding()">保存绑定</button>
          <button v-if="binding" :disabled="configLocked || loading" @click="saveBinding(group, true)">移除绑定</button>
          <button :disabled="configLocked || loading" @click="load()">{{ loading ? '正在刷新' : '刷新策略组' }}</button>
          <small>每 30 秒刷新 · 扫描期间暂停刷新</small>
        </div>
        </details>
        <p v-if="groupMissing" class="preflight warning" role="alert">目标策略组已删除或改名，请重新选择策略组；不会自动迁移绑定。</p>
        <div v-for="item in invalidBindings" :key="item.group" class="service-binding invalid-binding" role="status">
          <span>失效绑定：{{ item.group }} → {{ item.profile_id }}（{{ item.status === 'group_missing' ? '策略组不存在或不再是 Selector' : '服务模板不存在' }}）</span>
          <button :disabled="configLocked || loading" @click="saveBinding(item.group, true)">移除此失效绑定</button>
        </div>
        <div class="config-summary"><span><b>{{ profile?.label || '等待评分配置' }}</b><span v-if="preview?.ready"> · {{ preview.candidate_count }} 个候选 · {{ preview.batch_count }} 批</span></span><button :aria-expanded="showProfile" aria-controls="probe-profile" @click="showProfile = !showProfile">探测配置与地址<ChevronDown :size="15" :class="{expanded: showProfile}"/></button></div>

        <section v-if="profile && (showProfile || profile.requires_configuration)" id="probe-profile" class="profile-card" :class="{warning: profile.requires_configuration}">
          <div class="profile-copy">
            <small>本次评分配置 · {{ profile.id }}</small>
            <h2>{{ profile.label }}</h2>
            <p>{{ profile.description }}</p>
          </div>
          <dl>
            <div><dt>可达性</dt><dd>{{ profile.probe_count }} 个 HTTPS 探测</dd></div>
            <div><dt>严格验证</dt><dd>{{ profile.strict_probe_count ? (profile.strict_verification_available ? profile.strict_probe_count + ' 项可用' : profile.strict_probe_count + ' 项待配置') : '未请求' }}</dd></div>
            <div><dt>地区验证</dt><dd>{{ profile.expected_regions?.length ? profile.expected_regions.join(' / ') : '未配置预期出口' }}</dd></div>
            <div><dt>传输范围</dt><dd>{{ profile.transport_scope }}</dd></div>
          </dl>
          <section v-if="profile.targets?.length" class="profile-targets" aria-label="测试目标地址">
            <div class="target-heading"><b>测试目标地址</b><small>公开内置探测地址</small></div>
            <ul>
              <li v-for="target in profile.targets" :key="target.kind + target.name">
                <div><b>{{ probeKind(target.kind) }} · {{ target.name }}</b><small>期望 HTTP {{ target.expected_status }}</small></div>
                <code v-if="target.address_visible">{{ target.address }}</code>
                <span v-else class="private-target">私有目标已配置，不向 LAN 浏览器展示</span>
              </li>
            </ul>
          </section>
          <p v-if="profile.requires_configuration" class="profile-hint">{{ profile.setup_hint }}</p>
          <p v-if="profile.require_strict || profile.require_region" class="profile-scope">切换条件：{{ profile.require_strict ? '严格验证通过；' : '' }}{{ profile.require_region ? '出口地区匹配；' : '' }}</p>
          <p v-if="!profile.requires_configuration" class="profile-scope">评分衡量此服务的可达性、时延与稳定性；除非“严格验证”已通过，否则不把结果视为登录、解锁或播放证明。</p>
        </section>
        </div>

        <div v-if="!running && !groupMissing && discoveryValid && !profile?.requires_configuration && !preview?.ready" class="preflight warning"><b>当前不可扫描</b><span>{{ !group ? '未发现可选择的 Selector 策略组。' : preview?.reason || failure || '正在加载预检。' }}</span></div>

        <div v-if="running" class="progress">
          <div><b>{{ progress?.stage === 'refining' ? '复测阶段' : '初筛阶段' }} · 已完成探测任务 {{ progress?.completed || 0 }} / {{ progress?.total || 0 }} · 第 {{ progress?.current_batch || 0 }} / {{ progress?.total_batches || 0 }} 批</b><strong>{{ percent }}%</strong></div>
          <div class="progress-track" role="progressbar" aria-label="扫描进度" :aria-valuenow="percent" :aria-valuemin="0" :aria-valuemax="100"><span :style="{width: percent + '%'}"></span></div>
          <section><span>成功 <b>{{ progress?.succeeded || 0 }}</b></span><span>失败 <b>{{ progress?.failed || 0 }}</b></span><span>耗时 <b>{{ clock(progress?.elapsed_seconds) }}</b></span><span>预计剩余 <b>{{ clock(progress?.estimated_remaining_seconds) }}</b></span></section>
          <p><button @click="stop(true)">本批结束后停止</button><button class="danger" @click="stop(false)">立即停止</button></p>
        </div>

        <div class="result-workspace">
        <div v-if="running" class="scope-note" role="status">{{ connectionMode === 'live' ? '实时更新中' : connectionMode === 'paused' ? '页面刷新已暂停，后台扫描继续运行' : '定时刷新中' }}</div>
        <div class="scan-state" role="status"><span><CheckCircle2 v-if="scan?.status === 'complete'" :size="18"/><Radio v-else :size="18"/><b>{{ scanLabel }}</b> · {{ results.length }} 个节点</span><small>{{ scan ? scan.request.target_group + ' · ' + scan.profile.label + ' · ' : '' }}{{ running ? '结果返回即更新排名，验证后分数仍可能变化' : '扫描不改变当前节点' }}</small><span v-if="health?.mihomo_version === 'dev-mock'" class="fixture-label">示例数据</span></div>
        <div class="grid">
          <section class="ranking panel">
            <div class="head">
              <div><h2>实时排名</h2><p>{{ scan?.status === 'complete' ? '复测节点优先 · 性能满分 90 · 仅可选择符合条件的节点' : '结果返回即排序 · 扫描完成后可选择' }}</p></div>
            </div>
            <div v-if="results.length > 50" class="ranking-tools"><input v-model="resultQuery" type="search" placeholder="搜索扫描结果" aria-label="搜索扫描结果"><button :disabled="!results.some(row => row.name === current?.now)" @click="locateNode(current?.now)">定位当前节点</button><button :disabled="!candidate" @click="locateNode(candidate?.name)">定位候选</button></div>
            <ListPagination :page="resultPage" :pages="pageCount" :total="filteredResults.length" @change="resultPage = $event"/>
            <div v-if="!mobile" class="scroll desktop-ranking">
              <table aria-label="节点实时排名">
                <thead><tr><th>排名</th><th>节点 / Provider</th><th>地区</th><th class="numeric">成功率</th><th class="numeric">P95</th><th class="numeric">性能评分</th><th class="action-column">操作</th></tr></thead>
                <tbody>
                  <tr v-for="result in pageResults" :key="result.name" :class="{selected: candidate?.name === result.name}" @click="openDetails(result.name)">
                    <td class="rank">{{ result.rank }}</td>
                    <td>
                      <div class="node-name"><button class="node-focus" :aria-label="'查看 ' + result.name + ' 详情'" :aria-pressed="candidate?.name === result.name" @click.stop="openDetails(result.name)">{{ result.name }}</button><span v-if="current?.now === result.name" class="current-tag">当前</span></div><small>{{ result.provider }}</small>
                      <div class="state-row"><span class="state">{{ result.stage === 'refined' ? '已复测' : '仅初筛' }}</span>
                        <span :class="['state', statusTone(result.reachability_status)]">{{ statusLabel(result.reachability_status) }}</span>
                        <span v-if="statusTone(result.restriction_status) === 'bad'" class="state bad">{{ statusLabel(result.restriction_status) }}</span>
                        <span v-if="statusTone(result.strict_verification_status) === 'bad'" class="state bad">严格 · {{ statusLabel(result.strict_verification_status) }}</span>
                        <span v-if="statusTone(result.region_verification_status) === 'bad'" class="state bad">{{ statusLabel(result.region_verification_status) }}</span>
                      </div>
                    </td>
                    <td>{{ regionLabel(result.inferred_region) }}</td>
                    <td class="numeric">{{ percentLabel(result.success_rate) }}</td>
                    <td class="numeric">{{ latency(result.p95_ms) }}</td>
                    <td class="score numeric"><b>{{ result.score.toFixed(1) }}</b></td>
                    <td class="action-column"><button class="row-select" :aria-label="'选择 ' + result.name" :title="selectionReason(result) || '确认切换到 ' + result.name" :disabled="!!selectionReason(result)" @click.stop="choose(result)">选择</button></td>
                  </tr>
                  <tr v-if="!pageResults.length"><td colspan="7">{{ results.length ? '没有匹配节点，请调整搜索。' : '开始扫描后，部分结果会实时出现。' }}</td></tr>
                </tbody>
              </table>
            </div>
            <div v-else class="mobile-ranking">
              <button v-for="result in pageResults" :key="result.name" class="node-card" :class="{selected: candidate?.name === result.name}" :aria-label="'查看 ' + result.name + ' 详情'" @click="openDetails(result.name)">
                <span class="node-card-title"><strong>{{ result.rank }}. {{ result.name }}</strong><span v-if="current?.now === result.name" class="current-tag">当前</span><span class="score">{{ result.score.toFixed(1) }}<small>/90</small></span></span>
                <span class="node-card-meta">{{ result.provider || '未知 Provider' }} · {{ regionLabel(result.inferred_region) }}</span>
                <span class="node-card-metrics"><span>成功率 <b>{{ percentLabel(result.success_rate) }}</b></span><span>P95 <b>{{ latency(result.p95_ms) }}</b></span><span>{{ result.stage === 'refined' ? '已复测' : '仅初筛' }}</span></span>
                <span class="node-card-reason">{{ selectionReason(result) || '可确认切换 · 查看证据与详情' }}</span>
              </button>
              <p v-if="!pageResults.length" class="empty-detail">{{ results.length ? '没有匹配节点，请调整搜索。' : '开始扫描后，节点结果会显示在这里。' }}</p>
            </div>
          </section>

          <CandidateDetails v-if="!mobile" :state="state"/>
          <dialog v-else ref="detailDialog" class="candidate-drawer" aria-label="候选详情" @close="drawerOpen = false">
            <div class="drawer-heading"><b>候选详情</b><button aria-label="关闭候选详情" @click="closeDetails">关闭</button></div>
            <CandidateDetails :state="state"/>
          </dialog>
        </div>
        </div>
        <p v-if="preview && !running" class="scope-note">预计最多 {{ preview.probe_requests }} 次延迟请求；初筛 {{ preview.candidate_count }} 个节点，复测最多 {{ preview.refine_candidates }} 个（含当前节点预留名额）。</p>
        <p class="scope-note">评分反映可达性、时延与稳定性，不代表登录、解锁或播放验证通过。</p>
      </section>
</template>
