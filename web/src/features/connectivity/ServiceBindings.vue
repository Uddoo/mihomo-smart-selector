<script setup lang="ts">
import { t, formatDate } from '../../i18n/index'
import type { BoundGroup } from './bindings'
import NodeVerification from './NodeVerification.vue'

defineProps<{
  bindings: readonly BoundGroup[]
  available: boolean
  loading: boolean
  locked: boolean
}>()
defineEmits<{
  candidates: [group: string]
  scan: [id: string]
  verify: [group: string, profile: string]
}>()
const scanStatus = {
  running: '进行中',
  complete: '已完成',
  cancelled: '已停止',
  failed: '失败',
  interrupted: '已中断',
} as const
</script>

<template>
  <section
    class="service-bindings"
    :aria-label="t('关联策略组')"
  >
    <h3>{{ t('关联策略组') }}</h3>
    <p
      v-if="!available"
      class="binding-note"
      >{{ t(loading ? '正在读取策略组配置…' : '策略组配置暂不可用，请刷新配置后查看。') }}</p
    >
    <p
      v-else-if="!bindings.length"
      class="binding-note"
      >{{ t('尚无对应服务绑定，可在扫描工作台选择服务并保存绑定。') }}</p
    >
    <template v-else>
      <p class="binding-note">{{
        t('以下是 Controller 配置，不代表本次浏览器请求的实际路径。')
      }}</p>
      <div
        v-for="binding in bindings"
        :key="binding.group"
        class="bound-group"
      >
        <strong class="bound-group-name">{{ binding.group }}</strong>
        <template v-if="binding.valid">
          <dl class="binding-facts">
            <div
              ><dt>{{ t('当前配置选择') }}</dt
              ><dd>{{ binding.selection ?? t('选择尚未确认') }}</dd></div
            >
            <div
              ><dt>{{ t('最近节点扫描') }}</dt
              ><dd v-if="binding.latestScan"
                >{{ formatDate(binding.latestScan.started_at) }} ·
                {{ t(scanStatus[binding.latestScan.status]) }}</dd
              ><dd v-else>{{ t('最近记录中暂无该组与服务的扫描') }}</dd></div
            >
          </dl>
          <p
            v-if="binding.previousSelection !== undefined"
            class="binding-changed"
            >{{ t('节点已变化，可手动重测')
            }}<span>{{ binding.previousSelection }} → {{ binding.selection }}</span></p
          >
          <div class="binding-actions">
            <button
              :disabled="locked"
              @click="$emit('candidates', binding.group)"
              >{{ t('查看该组候选节点') }}</button
            >
            <button
              v-if="binding.latestScan"
              :disabled="locked"
              @click="$emit('scan', binding.latestScan.id)"
              >{{ t('查看最近扫描') }}</button
            >
          </div>
          <NodeVerification
            :binding="binding"
            :locked="locked"
            @prepare="(group, profile) => $emit('verify', group, profile)"
          />
        </template>
        <p
          v-else
          class="binding-note"
          >{{
            t(
              binding.status === 'group_missing'
                ? '绑定已失效：策略组不存在或不再是 Selector。'
                : '绑定已失效：服务模板不存在。',
            )
          }}</p
        >
      </div>
      <p
        v-if="locked"
        class="binding-note"
        >{{ t('扫描或配置操作进行中，完成后可查看关联记录。') }}</p
      >
    </template>
  </section>
</template>

<style scoped>
.service-bindings {
  border-top: 1px solid var(--line);
  margin-top: 12px;
  padding-top: 12px;
}
.service-bindings h3 {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
}
.binding-note {
  color: var(--muted);
  margin: 4px 0;
}
.bound-group {
  padding-block: 10px;
}
.bound-group + .bound-group {
  border-top: 1px solid var(--line);
}
.bound-group-name {
  display: block;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.binding-facts {
  margin: 6px 0;
}
.binding-facts > div {
  margin-top: 4px;
}
.binding-facts dt {
  color: var(--muted);
}
.binding-facts dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-variant-numeric: tabular-nums;
}
.binding-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.binding-actions button {
  min-height: 44px;
  font-size: 12px;
}
.binding-changed {
  color: var(--warning);
  margin: 8px 0;
}
.binding-changed span {
  display: block;
  overflow-wrap: anywhere;
}
</style>
