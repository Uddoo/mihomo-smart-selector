<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {ArrowRight} from '@lucide/vue'
import type {Workbench} from './useWorkbench'
const {state} = defineProps<{state: Workbench}>()
const {scan, candidate, configLocked, loading, retest, choose, selectionReason} = state
</script>

<template>
              <div class="candidate-action"><button v-if="scan?.status === 'complete'" :disabled="configLocked || loading" @click="retest">{{ t('复测此节点') }}</button><button class="primary" :disabled="!!selectionReason(candidate)" @click="choose()">{{ t('选择此节点') }}<ArrowRight :size="16"/></button><p>{{ translateMessage(selectionReason(candidate) || t('确认后切换，不自动切换')) }}</p></div>
</template>
