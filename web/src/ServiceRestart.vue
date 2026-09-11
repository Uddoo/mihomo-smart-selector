<script setup lang="ts">
import {onBeforeUnmount, shallowRef, watch} from 'vue'
import {RotateCw} from '@lucide/vue'
import {t, translateMessage} from './i18n'
import {useServiceRestart} from './useServiceRestart'

const props = defineProps<{disabled: boolean; pendingChanges: boolean}>()
const emit = defineEmits<{restarted: []; busyChange: [busy: boolean]}>()
const confirming = shallowRef(false)
const {state, busy, error, notice, restart, refresh} = useServiceRestart(() => emit('restarted'))
watch(busy, value => emit('busyChange', value))
watch(() => props.disabled, value => { if (value) confirming.value = false })
onBeforeUnmount(() => emit('busyChange', false))
function confirm() { confirming.value = false; void restart() }
</script>

<template>
  <section class="service-restart" :aria-label="t('服务重启')" :aria-busy="busy">
    <div class="restart-row">
      <div class="restart-copy">
        <h3>{{ t('服务重启') }}</h3>
        <p>{{ pendingChanges ? t('连接配置已保存，重启后生效。') : t('重新读取已保存的配置，并恢复后台监控。') }}</p>
      </div>
      <button type="button" :class="{primary: pendingChanges}" :disabled="disabled || busy || !state" @click="confirming = true">
        <RotateCw :size="16" :class="{spinning: busy}" aria-hidden="true"/>{{ busy ? t('正在重启服务…') : t('重启服务') }}
      </button>
    </div>
    <p v-if="disabled && !busy" class="settings-note">{{ t('请先保存或重新加载连接编辑，并结束正在运行的扫描。') }}</p>
    <div v-if="confirming" class="restart-confirm" role="group" :aria-label="t('确认重启服务')">
      <p>{{ t('页面会短暂断开，后台监控会暂停并在重启后恢复。Clash / Mihomo 内核继续运行。') }}</p>
      <div class="settings-actions">
        <button type="button" class="primary" @click="confirm">{{ t('确认重启服务') }}</button>
        <button type="button" @click="confirming = false">{{ t('取消') }}</button>
      </div>
    </div>
    <p v-if="busy" class="settings-note" role="status">{{ t('正在等待服务恢复，请保留此页面。') }}</p>
    <p v-if="notice" class="restart-success" role="status">{{ translateMessage(notice) }}</p>
    <div v-if="error" class="restart-error" role="alert">
      <span>{{ translateMessage(error) }}</span><button type="button" :disabled="busy" @click="refresh">{{ t('刷新服务状态') }}</button>
    </div>
  </section>
</template>

<style scoped>
.service-restart { margin-top: 20px; padding-top: 20px; border-top: 1px solid var(--line); }
.restart-row { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 14px; }
.restart-copy { flex: 1 1 280px; min-width: 0; }
.restart-copy h3 { margin: 0; font-size: 14px; }
.restart-copy p { margin: 6px 0 0; color: var(--muted); font-size: 13px; }
.restart-row button { display: inline-flex; align-items: center; justify-content: center; gap: 8px; }
.restart-confirm { margin-top: 14px; padding: 14px; background: var(--surface-muted); border-radius: 6px; }
.restart-confirm p { margin: 0 0 12px; max-width: 75ch; }
.restart-success { color: var(--green); font-size: 13px; }
.restart-error { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-top: 14px; color: var(--red); font-size: 13px; }
@media (max-width: 600px) { .restart-row > button { width: 100%; } }
</style>
