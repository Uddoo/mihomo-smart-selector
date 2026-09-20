<script setup lang="ts">
import MonitoringHub from './monitor/MonitoringHub.vue'
import type {Group, ServiceCatalog} from './models'
defineProps<{groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean}>()
const emit = defineEmits<{openScan: [group: string, profile: string]}>()
</script>

<template>
  <MonitoringHub :groups="groups" :services="services" :scan-locked="scanLocked" @open-scan="(group, profile) => emit('openScan', group, profile)"/>
</template>
