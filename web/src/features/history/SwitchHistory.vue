<script setup lang="ts">
import { History, RefreshCw, TriangleAlert } from '@lucide/vue'
import { t, translateMessage, formatDate } from '../../i18n/index'
import { operationMessage } from './selectionState'
import type { HistoryPageState } from './useSwitchHistory'
import HistoryToolbar from './HistoryToolbar.vue'
import HistoryRecord from './HistoryRecord.vue'
import { useHistoryView } from './useHistoryView'
import ListPagination from '../../shared/components/ListPagination.vue'
import './history.css'

const { state } = defineProps<{ state: HistoryPageState }>()
const {
  history,
  configLocked,
  reconcile,
  reconcilingId,
  reconcileFeedback,
  historyLoading,
  historyLoaded,
  historyError,
  historyUpdatedAt,
  refreshHistory,
} = state
const {
  query,
  group,
  source,
  status,
  groups,
  filtered,
  attentionCount,
  hasFilters,
  page,
  pageCount,
  visible,
  expanded,
  toggle,
  clear,
  showAttention,
} = useHistoryView(history)
</script>

<template>
  <section
    class="history-page"
    aria-labelledby="history-title"
  >
    <div
      v-if="attentionCount"
      class="history-attention"
    >
      <TriangleAlert
        :size="20"
        aria-hidden="true"
      />
      <div
        ><strong>{{ t('有 {count} 条切换需要核对', { count: attentionCount }) }}</strong
        ><p>{{ t('待确认或结果未知的切换会阻止同组继续切换。请先核对结果。') }}</p></div
      >
      <button
        :aria-pressed="status === 'needs-review'"
        @click="showAttention"
        >{{ t('只看待核对') }}</button
      >
    </div>
    <div class="history-surface">
      <div class="history-heading">
        <div
          ><h2 id="history-title">{{ t('节点切换记录') }}</h2
          ><p>{{ t('显示最近记录及所有待核对项，搜索筛选仅作用于已加载记录。') }}</p></div
        >
        <button
          class="history-refresh"
          :disabled="historyLoading || configLocked"
          @click="refreshHistory"
          ><RefreshCw
            :size="16"
            :class="{ 'history-spinning': historyLoading }"
            aria-hidden="true"
          />{{ historyLoading ? t('正在刷新') : t('刷新记录') }}</button
        >
      </div>
      <HistoryToolbar
        v-model:query="query"
        v-model:group="group"
        v-model:source="source"
        v-model:status="status"
        :groups="groups"
        :has-filters="hasFilters"
        @clear="clear"
      />
      <div class="history-meta">
        <span role="status">{{
          t('已加载 {total} 条 · 当前匹配 {count} 条', {
            total: history.length,
            count: filtered.length,
          })
        }}</span>
        <span v-if="historyUpdatedAt">{{
          t('更新于 {time}', { time: formatDate(historyUpdatedAt, 'time') })
        }}</span>
      </div>
      <div
        v-if="historyError"
        class="history-error"
        role="alert"
        ><TriangleAlert
          :size="18"
          aria-hidden="true"
        /><div
          ><strong>{{ t('选择历史读取失败') }}</strong
          ><p>{{ translateMessage(historyError) }}</p
          ><p v-if="history.length">{{ t('已保留上次加载的记录，可刷新重试。') }}</p></div
        ><button
          :disabled="historyLoading || configLocked"
          @click="refreshHistory"
          >{{ t('重试') }}</button
        ></div
      >
      <p
        v-if="reconcileFeedback"
        :class="['history-result', { 'is-error': reconcileFeedback.error }]"
        :role="reconcileFeedback.error ? 'alert' : 'status'"
        >{{
          t('记录 #{id}：{message}', {
            id: reconcileFeedback.id,
            message: reconcileFeedback.error
              ? translateMessage(reconcileFeedback.error)
              : reconcileFeedback.event
                ? translateMessage(operationMessage(reconcileFeedback.event))
                : '',
          })
        }}</p
      >
      <div
        v-if="(!historyLoaded || historyLoading) && !history.length"
        class="history-loading"
        role="status"
        ><p>{{ t('正在加载选择历史…') }}</p
        ><div
          v-for="n in 3"
          :key="n"
          class="history-skeleton"
          aria-hidden="true"
          ><span /><span /><span /></div
      ></div>
      <div
        v-else-if="!visible.length && (!historyError || history.length)"
        class="history-empty"
        role="status"
      >
        <History
          :size="28"
          :stroke-width="1.5"
          aria-hidden="true"
        />
        <h3>{{ hasFilters ? t('没有匹配的切换记录。') : t('尚无节点切换记录。') }}</h3>
        <p>{{
          hasFilters
            ? t('试试其他关键词，或清空筛选查看已加载记录。')
            : t('手动选择或监控自动切换后，记录会显示在这里。')
        }}</p>
        <button
          v-if="hasFilters"
          @click="clear"
          >{{ t('清空筛选') }}</button
        >
      </div>
      <table
        v-else-if="visible.length"
        class="history-table"
        role="table"
        :aria-label="t('节点切换记录')"
      >
        <thead role="rowgroup"
          ><tr role="row"
            ><th scope="col">{{ t('时间') }}</th
            ><th scope="col">{{ t('策略组') }}</th
            ><th scope="col">{{ t('节点变化') }}</th
            ><th scope="col">{{ t('来源') }}</th
            ><th scope="col">{{ t('结果') }}</th
            ><th scope="col">{{ t('操作') }}</th></tr
          ></thead
        >
        <tbody role="rowgroup"
          ><HistoryRecord
            v-for="item in visible"
            :key="item.id"
            :item="item"
            :expanded="expanded.has(item.id)"
            :busy="reconcilingId === item.id"
            :locked="configLocked || historyLoading"
            @toggle="toggle(item.id)"
            @reconcile="reconcile(item)"
        /></tbody>
      </table>
      <div
        v-if="filtered.length > 50"
        class="history-pagination"
        ><ListPagination
          :page="page"
          :pages="pageCount"
          :total="filtered.length"
          @change="page = $event"
      /></div>
      <p
        v-if="history.length"
        class="history-scope"
        >{{ t('核对结果只读取 Controller 当前状态并更新记录，不会重新执行切换。') }}</p
      >
    </div>
  </section>
</template>
