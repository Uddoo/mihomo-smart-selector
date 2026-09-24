<script setup lang="ts">
import { t } from '../../i18n/index'
import type { MonitorTask } from './monitoring'
defineProps<{
  tasks: MonitorTask[]
  selected: string
  dirty: string[]
  disabled: boolean
  canCreate: boolean
}>()
const emit = defineEmits<{ select: [id: string]; create: [] }>()
function status(task: MonitorTask) {
  if (!task.plan.enabled) return t('已暂停')
  if (!task.scheduled) return t('等待调度')
  if (!task.runtime) return t('状态更新中')
  if (task.runtime.suspended) return t('存储异常 · 已停止采样')
  if (task.runtime.issue) return t('等待环境恢复')
  return task.runtime.unhealthy
    ? t('{p0} 个需要关注', { p0: task.runtime.unhealthy })
    : t('后台监控中')
}
</script>

<template>
  <nav
    class="task-navigation"
    :aria-label="t('监控策略组')"
  >
    <div class="task-heading"
      ><div
        ><h2>{{ t('策略组监控') }}</h2
        ><p>{{ t('每组独立候选与自动切换，关闭页面后继续运行。') }}</p></div
      ><button
        :disabled="disabled || !canCreate"
        @click="emit('create')"
        >{{ t('新增监控策略组') }}</button
      ></div
    >
    <div class="task-list">
      <button
        v-for="task in tasks"
        :key="task.plan.task_id"
        class="task-choice"
        :class="{ selected: selected === task.plan.task_id }"
        :aria-pressed="selected === task.plan.task_id"
        :aria-label="t('查看策略组 {p0}', { p0: task.plan.group })"
        :disabled="disabled"
        @click="emit('select', task.plan.task_id)"
      >
        <span class="task-title"
          ><strong>{{ task.plan.group }}</strong
          ><span
            v-if="dirty.includes(task.plan.task_id)"
            class="draft-mark"
            >{{ t('未保存') }}</span
          ></span
        >
        <span class="task-meta"
          >{{ task.plan.profile_id }} · {{ t('{p0} 个候选', { p0: task.plan.nodes.length }) }}</span
        >
        <span class="task-current">{{ task.runtime?.current || t('等待 Controller 回读') }}</span>
        <span
          class="task-status"
          :class="{
            warning: task.runtime?.issue || task.runtime?.suspended || task.runtime?.unhealthy,
            neutral: !task.plan.enabled || !task.scheduled || !task.runtime,
          }"
          >{{ status(task)
          }}<span v-if="task.plan.auto_switch"> · {{ t('故障自动切换') }}</span></span
        >
      </button>
    </div>
    <p
      v-if="!canCreate && tasks.length"
      class="task-note"
      >{{ t('所有可选策略组均已有监控任务。') }}</p
    >
  </nav>
</template>

<style scoped>
.task-navigation {
  display: block;
  min-width: 0;
}
.task-heading {
  display: flex;
  gap: var(--space-4);
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
}
.task-heading h2 {
  font-size: 18px;
  margin: 0 0 8px;
}
.task-heading p,
.task-note {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.7;
  margin: 0;
}
.task-heading button {
  min-height: 44px;
  flex-shrink: 0;
}
.task-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--space-3);
}
.task-choice {
  white-space: normal;
  justify-content: flex-start;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  text-align: left;
  gap: 8px;
  min-width: 0;
  padding: var(--space-4);
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
}
.task-choice.selected {
  border-color: var(--accent);
  background: var(--selection-bg);
}
.task-title {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}
.task-title strong {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.task-meta,
.task-status,
.draft-mark {
  font-size: 12px;
  line-height: 1.6;
}
.task-meta {
  color: var(--muted);
}
.task-current {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.task-status {
  color: var(--green);
  margin-top: auto;
}
.task-status.warning,
.draft-mark {
  color: var(--warning);
}
.task-status.neutral {
  color: var(--muted);
}
.task-note {
  margin-top: 12px;
}
@media (max-width: 640px) {
  .task-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .task-heading button {
    width: 100%;
  }
  .task-list {
    grid-template-columns: 1fr;
  }
  .task-choice {
    gap: 6px;
  }
}
</style>
