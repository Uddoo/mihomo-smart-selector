<script setup lang="ts">
import MonitoringHub from './MonitoringHub.vue'
import type { Group, ServiceCatalog } from '../../shared/types/models'
defineProps<{ groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean }>()
const emit = defineEmits<{ openScan: [group: string, profile: string] }>()
</script>

<template>
  <MonitoringHub
    :groups="groups"
    :services="services"
    :scan-locked="scanLocked"
    @open-scan="(group, profile) => emit('openScan', group, profile)"
  />
</template>
