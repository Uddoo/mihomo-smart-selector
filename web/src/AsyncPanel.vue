<script setup lang="ts">
import {onBeforeUnmount, onMounted, shallowRef} from 'vue'
import type {Component} from 'vue'
import {t} from './i18n'

defineOptions({inheritAttrs: false})
const props = defineProps<{loader: () => Promise<{default: Component}>}>()
const view = shallowRef<Component>()
const failed = shallowRef(false)
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined

async function load() {
  const current = ++generation
  failed.value = false
  clearTimeout(timer)
  // Imports cannot be cancelled; ignore late completions after timeout or navigation.
  timer = setTimeout(() => { if (current === generation) { generation++; failed.value = true } }, 15000)
  try {
    const module = await props.loader()
    if (current === generation) view.value = module.default
  } catch {
    if (current === generation) failed.value = true
  } finally {
    if (current === generation) clearTimeout(timer)
  }
}
function reloadPage() { window.location.reload() }
onMounted(load)
onBeforeUnmount(() => { generation++; clearTimeout(timer) })
</script>

<template>
  <component :is="view" v-if="view" v-bind="$attrs"><slot/></component>
  <section v-else-if="failed" class="page-load-state" role="alert">
    <h2>{{ t('页面加载失败') }}</h2>
    <p>{{ t('请检查网络后重试。如果服务刚刚更新，请重新加载页面。') }}</p>
    <div class="settings-actions"><button class="primary" @click="load">{{ t('重试加载') }}</button><button @click="reloadPage">{{ t('重新加载页面') }}</button></div>
  </section>
  <section v-else class="page-load-state" role="status" aria-busy="true"><p>{{ t('正在加载页面…') }}</p></section>
</template>

<style scoped>
.page-load-state { padding:24px; margin-bottom:20px; border:1px solid var(--line); border-radius:var(--radius-md); background:var(--surface); }
.page-load-state h2 { font-size:18px; }
.page-load-state p { color:var(--muted); max-width:65ch; }
.page-load-state[role="status"] p { margin:0; }
</style>
