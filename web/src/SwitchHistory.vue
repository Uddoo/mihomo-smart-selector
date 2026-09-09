<script setup lang="ts">
import {operationLabel} from './selectionState'
import type {Workbench} from './useWorkbench'
const {state} = defineProps<{state: Workbench}>()
const { history, switching, reconcile } = state

</script>

<template>
      <section class="panel"><h2>节点切换记录</h2><article v-for="item in history" :key="item.id"><small>{{ new Date(item.created_at).toLocaleString() }} · {{ item.scan_id.startsWith('monitor:') ? '监控自动切换' : '手动选择' }}</small><b>{{ item.previous || '—' }} → {{ item.selected }}</b><span>{{ item.group }} · {{ operationLabel(item.status) }}</span><p>{{ item.reason }}</p><button v-if="['pending','unknown'].includes(item.status) || !item.audit_persisted" :disabled="switching" @click="reconcile(item)">核对结果</button></article><p v-if="!history.length">尚无节点切换记录。</p></section>
</template>
