<script setup lang="ts">
import {computed, ref, shallowRef} from 'vue'
import {t, translateMessage} from '../i18n'
import {useMonitorTasks} from './useMonitorTasks'
import {draftChanged, emptyView, monitorWorkload} from './taskState'
import type {MonitorViewState, MonitorDraft} from './taskState'
import type {Group, ServiceCatalog} from '../models'
import type {MonitorPlan} from '../monitoring'
import MonitorTaskList from './MonitorTaskList.vue'
import MonitorSharedCapacity from './MonitorSharedCapacity.vue'
import MonitorTaskView from './MonitorTaskView.vue'
import MonitorPlanEditor from '../MonitorPlanEditor.vue'

const props = defineProps<{groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean}>()
const emit = defineEmits<{openScan: [group: string, profile: string]}>()
const {tasks, scheduler, retention, selected, failure, capacityFailure, initialized, choose, refresh, saved, updateRetention} = useMonitorTasks()
const states = ref<Record<string, MonitorViewState>>({})
const busy = shallowRef(false)
const current = computed(() => tasks.value.find(task => task.plan.task_id === selected.value))
const dirty = computed(() => tasks.value.filter(task => draftChanged(states.value[task.plan.task_id]?.draft, task.plan)).map(task => task.plan.task_id))
const canCreate = computed(() => props.groups.some(group => !tasks.value.some(task => task.plan.group === group.name)))
const workload = computed(() => monitorWorkload(tasks.value, props.services))
const state = computed(() => states.value[selected.value] || emptyView())
function saveState(id: string, value: MonitorViewState) { states.value[id] = value }
function saveNewDraft(value: MonitorDraft) { saveState('new', {...(states.value.new || emptyView()), draft: value}) }
function created(plan: MonitorPlan) { delete states.value.new; saved(plan) }
</script>

<template>
  <div class="monitor-hub">
    <div v-if="failure" class="notice error" role="alert">{{ translateMessage(failure) }} <button @click="refresh">{{ t('重新读取') }}</button></div>
    <p v-if="!initialized && !failure" role="status">{{ t('正在读取监控策略组…') }}</p>
    <template v-if="initialized">
      <MonitorTaskList :tasks="tasks" :selected="selected" :dirty="dirty" :disabled="busy" :can-create="canCreate" @select="choose" @create="choose('new')"/>
      <MonitorSharedCapacity :tasks="tasks" :policy="retention" :scheduler="scheduler" :services="services" :failure="capacityFailure"/>
      <MonitorPlanEditor v-if="selected === 'new'" key="new" :draft="states.new?.draft" :tasks="tasks" :scheduler="scheduler" :groups="groups" :services="services" :disabled="busy" :retention-policy="retention" @draft="saveNewDraft" @busy="busy = $event" @saved="created" @cancel="delete states.new; choose(tasks[0]?.plan.task_id || 'new')"/>
      <MonitorTaskView v-else-if="current" :key="current.plan.task_id" :task="current" :tasks="tasks" :scheduler="scheduler" :view-state="state" :groups="groups" :services="services" :scan-locked="scanLocked" :retention="retention" @retention-read="updateRetention" :total-candidates="workload.candidates" :total-tasks="workload.groups" @view-state="saveState" @busy="busy = $event" @saved="saved" @open-scan="(group, profile) => emit('openScan', group, profile)"/>
      <p class="draft-note">{{ t('切换策略组保留未保存草稿；离开此页会清除草稿。') }}</p>
    </template>
  </div>
</template>

<style scoped>
.monitor-hub{display:grid;gap:var(--space-5);min-width:0}.draft-note{color:var(--muted);font-size:12px;line-height:1.7;margin:0}
</style>
