<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { t, translateMessage, formatRegion } from '../../i18n/index'
import { ipIntelligencePanelID, requestPanelFocus } from '../../app/panelFocus'
import { monitorTime } from './monitoring'
import type { MonitorRow } from './monitoring'
import { egressFresh, egressStatusKey, intelligenceSources } from './egress'
import MonitorIPIntelligence from './MonitorIPIntelligence.vue'

const props = defineProps<{ rows: MonitorRow[]; current: string; now: string; active: boolean }>()
const expanded = shallowRef(false)
const rows = computed(() =>
  props.rows.map((row) => {
    const value = row.egress
    const status = egressStatusKey(value, props.now, props.active)
    // A paused task keeps valid evidence; only aged observations and real
    // errors keep the warning tone.
    const stale = !egressFresh(value, props.now, props.active)
    const warning = !!value && stale && !(value.status === 'ok' && !props.active)
    return { row, value, warning, status, sources: intelligenceSources(value) }
  }),
)
const visibleRows = computed(() =>
  expanded.value ? rows.value : rows.value.filter((item) => item.row.name === props.current),
)
const otherCount = computed(
  () => rows.value.filter((item) => item.row.name !== props.current).length,
)
</script>

<template>
  <section
    class="monitor-panel egress-panel"
    :aria-label="t('ChatGPT 出口与 IP 信誉')"
  >
    <div class="monitor-heading"
      ><div
        ><h3>{{ t('ChatGPT 出口与 IP 信誉') }}</h3>
        <p>{{
          t(
            '经专用探测路径读取每个节点的 ChatGPT 出口；成功采样后约每 10 分钟更新，各来源情报独立缓存 24 小时。',
          )
        }}</p> </div
      ><a
        class="egress-settings-link"
        href="#/settings"
        @click="requestPanelFocus(ipIntelligencePanelID)"
        >{{ t('前往 IP 情报设置') }}</a
      ></div
    >
    <p
      v-if="!rows.length"
      class="monitor-note"
      >{{ t('等待成功采样') }}</p
    >
    <p
      v-if="rows.length && !visibleRows.length"
      class="monitor-note"
      >{{ t('当前选择不在监控列表，展开查看候选节点的出口。') }}</p
    >
    <div
      id="monitor-egress-list"
      class="egress-list"
    >
      <article
        v-for="item in visibleRows"
        :key="item.row.id"
        class="egress-row"
        :aria-label="item.row.name"
      >
        <div class="egress-summary">
          <div class="egress-identity">
            <strong>{{ item.row.name }}</strong>
            <span
              v-if="item.row.name === current"
              class="current-tag"
              >{{ t('策略组当前选择') }}</span
            >
            <small :class="{ 'egress-warning': item.warning }">{{ t(item.status) }}</small>
          </div>
          <div class="egress-address">
            <small>{{ t('ChatGPT 出口 IP') }}</small
            ><span class="egress-ip">{{ item.value?.ip || '—' }}</span>
            <small v-if="item.value?.country_code">{{
              formatRegion(item.value.country_code)
            }}</small>
            <small v-if="item.value?.status === 'not_configured'">{{
              t('请在扫描设置中配置并启用真实出口地区验证。')
            }}</small>
            <small
              v-if="item.value?.error"
              class="egress-warning"
              >{{ translateMessage(item.value.error) }}</small
            >
            <small v-if="item.value?.status === 'ok'">{{
              t('出口检测：{p0}', { p0: monitorTime(item.value.observed_at) })
            }}</small>
          </div>
        </div>
        <MonitorIPIntelligence
          v-if="item.sources.length"
          :sources="item.sources"
          :now="now"
        />
        <p
          v-else
          class="monitor-note"
          >{{ t('等待出口检测') }}</p
        >
      </article>
    </div>
    <button
      v-if="otherCount"
      class="egress-expand"
      :aria-expanded="expanded"
      aria-controls="monitor-egress-list"
      @click="expanded = !expanded"
      >{{
        expanded ? t('收起其他节点') : t('查看其他节点出口（{p0}）', { p0: otherCount })
      }}</button
    >
    <details class="egress-explanation"
      ><summary>{{ t('数据来源与配置') }}</summary>
      <p>{{
        t(
          '各来源独立展示。IPQualityScore 与 Scamalytics 的信誉分为 100 − 原始风险分；AbuseIPDB 展示举报滥用置信分；ipapi.is 展示属性与网络滥用比例，不换算综合分。',
        )
      }}</p>
      <p>{{
        t('这些结果不代表 OpenAI 官方评分、帐号状态或封禁概率；不同来源的分数不能直接平均。')
      }}</p>
      <p>{{
        t(
          'IP 信誉独立展示，不参与 HTTPS 健康分、节点排名和故障自动切换。暂停监控或出口观测超时后，旧结果仅供查阅。',
        )
      }}</p>
      <p>{{ t('在设置页的“IP 情报服务”中填写密钥；也可使用服务端环境变量或密钥文件。') }}</p>
      <p>{{
        t(
          '只提交出口 IP、必要的帐号凭据及固定查询参数；密钥保留在后端。每个来源独立缓存和退避，缓存与最新出口观测在服务重启后重新采集。',
        )
      }}</p>
    </details>
  </section>
</template>

<style scoped>
.egress-panel > .monitor-heading {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px 24px;
  margin-bottom: 16px;
}
.egress-panel .monitor-heading h3 {
  margin: 0;
  font-size: 15px;
}
.egress-panel .monitor-heading p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--muted);
  max-width: 75ch;
}
.egress-panel > .monitor-heading > div {
  flex: 1;
  min-width: min(100%, 300px);
}
.egress-settings-link {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--muted);
  font-size: 12px;
  text-underline-offset: 3px;
}
.egress-settings-link:hover {
  color: var(--text);
}
.egress-list {
  display: grid;
}
.egress-expand {
  min-height: 44px;
  margin: 0 0 12px;
}
.egress-row {
  padding: 16px 0;
  border-top: 1px solid var(--line);
}
.egress-summary {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) minmax(180px, 2fr);
  gap: 16px;
  margin-bottom: 16px;
}
.egress-identity,
.egress-address {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-width: 0;
  overflow-wrap: anywhere;
}
.egress-row small {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.egress-ip {
  color: var(--text);
  font-size: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.egress-row .egress-warning {
  color: var(--warning);
}
.egress-explanation {
  border-top: 1px solid var(--line);
  padding-top: 12px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
.egress-explanation summary {
  cursor: pointer;
  min-height: 40px;
  display: list-item;
}
.egress-explanation summary:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 4px;
}
@media (max-width: 600px) {
  .egress-summary {
    grid-template-columns: 1fr;
  }
}
</style>
