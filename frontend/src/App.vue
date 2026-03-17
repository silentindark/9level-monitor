<template>
  <HeaderBar />

  <div class="container">
    <!-- Tabs -->
    <div class="tabs">
      <div class="tab" :class="{ active: tab === 'calls' }" @click="tab = 'calls'">
        Chamadas ({{ calls.length }} canais)
      </div>
      <div class="tab" :class="{ active: tab === 'endpoints' }" @click="tab = 'endpoints'">
        Endpoints ({{ totalEndpoints }})
      </div>
      <div class="tab" :class="{ active: tab === 'security' }" @click="tab = 'security'" :style="securitySummary.total_events > 0 ? 'color: var(--red)' : ''">
        Seguran&#231;a ({{ securitySummary.total_events || 0 }})
      </div>
      <div class="tab" :class="{ active: tab === 'charts' }" @click="tab = 'charts'; $nextTick(() => chartsRef?.loadCharts())">
        Gr&#225;ficos
      </div>
      <div class="tab" :class="{ active: tab === 'history' }" @click="tab = 'history'; $nextTick(() => historyRef?.loadHistoryCalls())">
        Hist&#243;rico
      </div>
      <div class="tab" :class="{ active: tab === 'admin' }" @click="tab = 'admin'">
        Administra&#231;&#227;o
      </div>
    </div>

    <!-- Tab content -->
    <MonitorTab v-if="tab === 'calls'" />
    <EndpointsTab v-if="tab === 'endpoints'" />
    <SecurityTab v-if="tab === 'security'" />
    <ChartsTab v-if="tab === 'charts'" ref="chartsRef" />
    <HistoryTab v-if="tab === 'history'" ref="historyRef" />
    <AdminTab v-if="tab === 'admin'" />

    <div class="footer">9LEVEL MONITOR V2.0</div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import {
  calls, totalEndpoints, securitySummary,
  initialFetch, connectSSE, disconnectSSE
} from './composables/useStore'
import HeaderBar from './components/HeaderBar.vue'
import MonitorTab from './components/MonitorTab.vue'
import EndpointsTab from './components/EndpointsTab.vue'
import SecurityTab from './components/SecurityTab.vue'
import ChartsTab from './components/ChartsTab.vue'
import HistoryTab from './components/HistoryTab.vue'
import AdminTab from './components/AdminTab.vue'

const tab = ref('calls')
const chartsRef = ref(null)
const historyRef = ref(null)

onMounted(() => {
  initialFetch()  // one-time REST fetch for immediate data
  connectSSE()    // persistent connection for all real-time updates
})

onUnmounted(() => {
  disconnectSSE()
})
</script>
