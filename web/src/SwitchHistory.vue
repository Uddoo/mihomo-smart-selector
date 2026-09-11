<script setup lang="ts">
import {t, translateMessage, formatDate} from './i18n'
import {operationLabel} from './selectionState'
import type {Workbench} from './useWorkbench'
const {state} = defineProps<{state: Workbench}>()
const { history, switching, reconcile } = state

</script>

<template>
      <section class="panel"><h2>{{ t('节点切换记录') }}</h2><article v-for="item in history" :key="item.id"><small>{{ formatDate(new Date(item.created_at)) }} · {{ item.scan_id.startsWith('monitor:') ? t('监控自动切换') : t('手动选择') }}</small><b>{{ item.previous || '—' }} → {{ item.selected }}</b><span>{{ item.group }} · {{ translateMessage(operationLabel(item.status)) }}</span><p>{{ translateMessage(item.reason) }}</p><button v-if="['pending','unknown'].includes(item.status) || !item.audit_persisted" :disabled="switching" @click="reconcile(item)">{{ t('核对结果') }}</button></article><p v-if="!history.length">{{ t('尚无节点切换记录。') }}</p></section>
</template>
