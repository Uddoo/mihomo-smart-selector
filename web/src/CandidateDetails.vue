<script setup lang="ts">
import {computed} from 'vue'
import CandidateActions from './CandidateActions.vue'
import {candidateComparison} from './ranking'
import type {Workbench} from './useWorkbench'
import {ChevronDown} from '@lucide/vue'
const {state} = defineProps<{state: Workbench}>()
const { loading, now, current, best, candidate, currentResult, configLocked, retest, selectionReason, choose, regionLabel, percentLabel, latency, points, statusLabel, statusTone, scan, running, hasJitterEvidence, evidence } = state

const comparison = computed(() => candidateComparison(candidate.value, currentResult.value, now.value))
</script>

<template>
          <aside class="panel compare" aria-label="候选详情">
            <h2>候选详情</h2>
            <template v-if="candidate">
              <div class="candidate-heading"><span class="candidate-tag">{{ candidate.name === best?.name ? (running ? '暂列第一' : '最高评分') : '已选候选' }}</span><h3>{{ candidate.name }}</h3><p>{{ candidate.provider || '未知 Provider' }} · {{ regionLabel(candidate.inferred_region) }}</p></div>
              <section class="decision-summary" aria-label="候选决策摘要">
                <small>与当前节点的本次测量比较</small><strong>{{ comparison }}</strong>
                <p>{{ evidence(candidate) }}</p>
                <p :class="selectionReason(candidate) ? 'bad' : 'good'">{{ selectionReason(candidate) || (current?.now === candidate.name ? '当前节点，可继续观察或复测' : '符合当前切换条件，确认后执行') }}</p>
              </section>
              <CandidateActions class="desktop-candidate-actions" :state="state"/>
              <div class="candidate-score"><div><b>性能评分</b><span><strong>{{ candidate.score.toFixed(1) }}</strong> / 90</span></div><meter min="0" max="90" :value="candidate.score" aria-label="候选性能评分"/></div>
              <div class="current-node"><span>当前节点</span><b>{{ current?.now || '—' }}</b></div>
              <table class="comparison-table" aria-label="当前与候选指标"><thead><tr><th></th><th>当前</th><th>候选</th></tr></thead><tbody><tr><th>P95 时延</th><td>{{ currentResult ? latency(currentResult.p95_ms) : '本次未测' }}</td><td>{{ latency(candidate.p95_ms) }}</td></tr><tr><th>成功率</th><td>{{ currentResult ? percentLabel(currentResult.success_rate) : '本次未测' }}</td><td>{{ percentLabel(candidate.success_rate) }}</td></tr></tbody></table>
              <p class="settings-note">采样时间：{{ candidate.measured_at ? new Date(candidate.measured_at).toLocaleString() : '历史记录未提供' }}<br>有效至：{{ candidate.expires_at ? new Date(candidate.expires_at).toLocaleTimeString() : '—' }}</p>
              <p v-if="currentResult && currentResult.name !== candidate.name" class="settings-note">当前节点证据：{{ evidence(currentResult) }}</p>
              <section class="assessment"><h3>服务验证</h3><dl>
                <div><dt>可达性</dt><dd :class="statusTone(candidate.reachability_status)">{{ statusLabel(candidate.reachability_status) }}</dd></div>
                <div><dt>严格验证</dt><dd :class="statusTone(candidate.strict_verification_status)">{{ statusLabel(candidate.strict_verification_status) }}</dd></div>
                <div><dt>地区验证</dt><dd :class="statusTone(candidate.region_verification_status)">{{ statusLabel(candidate.region_verification_status) }}</dd></div>
                <div><dt>服务限制</dt><dd :class="statusTone(candidate.restriction_status)">{{ statusLabel(candidate.restriction_status) }}</dd></div>
              </dl></section>
              <details class="score-details"><summary>性能得分拆解<ChevronDown :size="15"/></summary><dl class="breakdown"><div><dt>可靠性</dt><dd>{{ points(candidate.score_breakdown?.reliability) }} / 40</dd></div><div><dt>P50</dt><dd>{{ points(candidate.score_breakdown?.p50) }} / 15</dd></div><div><dt>P95</dt><dd>{{ points(candidate.score_breakdown?.p95) }} / 20</dd></div><div><dt>抖动</dt><dd>{{ hasJitterEvidence(candidate) ? points(candidate.score_breakdown?.jitter) + ' / 10' : '样本不足 · 0 / 10' }}</dd></div><div><dt>地区</dt><dd>{{ points(candidate.score_breakdown?.region) }} / 5</dd></div></dl><p>传输范围：{{ candidate.transport_status }}</p></details>
              <CandidateActions class="mobile-candidate-actions" :state="state"/>
            </template>
            <p v-else class="empty-detail">{{ running ? '等待第一个节点完成检测，结果将实时出现。' : '开始扫描，或点击排名中的节点查看详情。' }}</p>
          </aside>
</template>
