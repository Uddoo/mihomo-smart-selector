<script setup lang="ts">
import {watch} from 'vue'
import {usePagination} from './usePagination'
import ListPagination from './ListPagination.vue'
import type {Workbench} from './useWorkbench'
const {state} = defineProps<{state: Workbench}>()
const { nodes, query, selected, visibleNodes, regionLabel } = state

const {page, pageCount, visible} = usePagination(visibleNodes)
watch([query, state.areas, state.providerSet], () => { page.value = 1 })
</script>

<template>
      <section>
        <div class="toolbar"><input v-model="query" placeholder="搜索节点"><b>{{ visibleNodes.length }} / {{ nodes.length }} 个叶子节点</b></div>
        <ListPagination :page="page" :pages="pageCount" :total="visibleNodes.length" @change="page = $event"/>
        <div class="nodegrid"><section class="panel scroll"><table><thead><tr><th>节点</th><th>地区</th><th>Provider</th><th>协议</th></tr></thead><tbody><tr v-for="node in visible" :key="node.name" @click="selected = node"><td><button class="node-focus" @click.stop="selected = node">{{ node.name }}</button></td><td>{{ regionLabel(node.inferred_region) }}</td><td>{{ node.provider || '—' }}</td><td>{{ node.protocol || '—' }}</td></tr></tbody></table></section><aside class="panel"><h2>节点详情</h2><template v-if="selected"><b>{{ selected.name }}</b><p>{{ regionLabel(selected.inferred_region) }} · {{ selected.region_source }}</p><p>Provider：{{ selected.provider || '—' }}</p><p>协议：{{ selected.protocol || '—' }}</p></template><p v-else>选择节点查看地区推断。</p></aside></div>
      </section>
</template>
