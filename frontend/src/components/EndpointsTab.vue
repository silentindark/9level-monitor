<template>
  <div class="search">
    <input v-model="endpointSearch" placeholder="Buscar endpoint..." />
    <select v-model="endpointFilter">
      <option value="all">Todos</option>
      <option value="ONLINE">Online</option>
      <option value="OFFLINE">Offline</option>
    </select>
  </div>
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>Endpoint</th><th>Estado</th><th>Contato</th><th>Status</th><th>RTT</th><th>Hist&#243;rico</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="ep in filteredEndpoints" :key="ep.name">
          <tr>
            <td>{{ ep.name }}</td>
            <td><span class="badge" :class="ep.state.toLowerCase()">{{ ep.state }}</span></td>
            <td>{{ ep.contacts && ep.contacts.length > 0 ? ep.contacts[0].uri : '-' }}</td>
            <td>{{ ep.contacts && ep.contacts.length > 0 ? ep.contacts[0].status : '-' }}</td>
            <td>{{ ep.contacts && ep.contacts.length > 0 ? (ep.contacts[0].rtt_us / 1000).toFixed(1) + ' ms' : '-' }}</td>
            <td><button class="ep-hist-btn" @click="toggleEpHistory(ep.name)">{{ epHistoryOpen === ep.name ? 'Fechar' : 'Ver' }}</button></td>
          </tr>
          <tr v-if="epHistoryOpen === ep.name" class="timeline-row">
            <td colspan="6">
              <div v-if="epHistoryData.length === 0" style="color:var(--text-muted);font-size:12px;padding:8px 0">Sem mudan&#231;as registradas</div>
              <div v-else class="timeline-list">
                <div class="timeline-entry" v-for="h in epHistoryData" :key="h.id">
                  <span class="time">{{ formatDateTime(h.timestamp) }}</span>
                  <span class="badge" :class="h.old_state === 'ONLINE' ? 'online' : 'offline'">{{ h.old_state }}</span>
                  <span style="color:var(--text-muted)">&rarr;</span>
                  <span class="badge" :class="h.new_state === 'ONLINE' ? 'online' : 'offline'">{{ h.new_state }}</span>
                </div>
              </div>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { endpoints, formatDateTime } from '../composables/useStore'

const BASE = import.meta.env.BASE_URL.replace(/\/$/, '')

const endpointSearch = ref('')
const endpointFilter = ref('all')
const epHistoryOpen = ref('')
const epHistoryData = ref([])

const filteredEndpoints = computed(() => {
  let list = endpoints.value
  if (endpointFilter.value !== 'all') {
    list = list.filter(ep => ep.state === endpointFilter.value)
  }
  if (endpointSearch.value) {
    const q = endpointSearch.value.toLowerCase()
    list = list.filter(ep =>
      ep.name.toLowerCase().includes(q) ||
      (ep.contacts && ep.contacts.some(c => c.uri.toLowerCase().includes(q)))
    )
  }
  list = [...list].sort((a, b) => {
    if (a.state === 'ONLINE' && b.state !== 'ONLINE') return -1
    if (a.state !== 'ONLINE' && b.state === 'ONLINE') return 1
    const rttA = a.contacts && a.contacts.length > 0 ? (a.contacts[0].rtt_us || 0) : 0
    const rttB = b.contacts && b.contacts.length > 0 ? (b.contacts[0].rtt_us || 0) : 0
    if (rttB !== rttA) return rttB - rttA
    return a.name.localeCompare(b.name, undefined, { numeric: true })
  })
  return list
})

async function toggleEpHistory(name) {
  if (epHistoryOpen.value === name) {
    epHistoryOpen.value = ''
    epHistoryData.value = []
    return
  }
  epHistoryOpen.value = name
  try {
    const res = await fetch(BASE + '/api/v1/history/endpoints?endpoint=' + encodeURIComponent(name) + '&per_page=50')
    if (res.ok) {
      const data = await res.json()
      epHistoryData.value = data.items || []
    } else {
      epHistoryData.value = []
    }
  } catch (e) { epHistoryData.value = [] }
}
</script>
