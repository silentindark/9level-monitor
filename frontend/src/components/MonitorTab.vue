<template>
  <!-- Summary cards -->
  <div class="cards">
    <div class="card">
      <div class="label">Chamadas Ativas</div>
      <div class="value accent">{{ summary.active_calls || 0 }}</div>
    </div>
    <div class="card">
      <div class="label">Pico de Chamadas</div>
      <div class="value yellow">{{ summary.peak_calls || 0 }}</div>
    </div>
    <div class="card">
      <div class="label">Endpoints Online</div>
      <div class="value green">{{ summary.registered_endpoints || 0 }}</div>
    </div>
    <div class="card">
      <div class="label">Total Endpoints</div>
      <div class="value">{{ totalEndpoints }}</div>
    </div>
    <div class="card">
      <div class="label">MES M&#233;dio (RX)</div>
      <div class="value" :class="mesClass(summary.avg_rxmes)">{{ (summary.avg_rxmes || 0).toFixed(2) }}</div>
    </div>
    <div class="card">
      <div class="label">Uptime</div>
      <div class="value" style="font-size:20px">{{ formatUptime(summary.uptime_seconds) }}</div>
    </div>
  </div>

  <!-- Call filters -->
  <div class="filters-bar" v-if="calls.length > 0">
    <div class="filter-group">
      <label>Busca</label>
      <input v-model="callSearch" placeholder="Canal, caller, callee..." style="width:220px">
    </div>
    <div class="filter-group">
      <label>Tronco</label>
      <select v-model="callTrunkFilter">
        <option value="">Todos</option>
        <option v-for="t in detectedTrunks" :key="t" :value="t">{{ t }}</option>
      </select>
    </div>
    <div class="filter-group">
      <label>Dire&#231;&#227;o</label>
      <select v-model="callDirectionFilter">
        <option value="">Todas</option>
        <option value="inbound">Entrantes</option>
        <option value="outbound">Sa&#237;das</option>
      </select>
    </div>
  </div>

  <div v-if="calls.length === 0" class="card" style="text-align:center;padding:40px">
    Nenhuma chamada ativa
  </div>
  <div v-else-if="filteredCallTree.length === 0" class="card" style="text-align:center;padding:40px;color:var(--text-muted)">
    Nenhuma chamada corresponde aos filtros
  </div>
  <div v-else class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>Canal</th><th>Origem</th><th>Destino</th><th>Dura&#231;&#227;o</th><th>Codec</th>
          <th>RX Jitter</th><th>TX Jitter</th><th>RX Loss</th><th>RTT</th><th>MES</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="group in filteredCallTree" :key="group.root.channel">
          <tr class="call-group call-root">
            <td>{{ group.root.channel }}</td>
            <td>{{ group.root.caller }}</td>
            <td>{{ group.root.callee }}</td>
            <td>{{ formatDuration(group.root.duration_seconds) }}</td>
            <td>{{ group.root.codec }}</td>
            <td>{{ group.root.rtp ? Math.round(group.root.rtp.rxjitter) + ' ms' : 'N/A' }}</td>
            <td>{{ group.root.rtp ? Math.round(group.root.rtp.txjitter) + ' ms' : 'N/A' }}</td>
            <td>{{ group.root.rtp ? group.root.rtp.rxploss : 'N/A' }}</td>
            <td>{{ group.root.rtp ? group.root.rtp.rtt.toFixed(2) + ' ms' : 'N/A' }}</td>
            <td>
              <template v-if="group.root.rtp">
                <span class="quality-bar"><span class="fill" :class="mesBarClass(group.root.rtp.rxmes)" :style="{ width: mesBarWidth(group.root.rtp.rxmes) }"></span></span>
                {{ group.root.rtp.rxmes.toFixed(2) }}
              </template>
              <template v-else>N/A</template>
            </td>
          </tr>
          <tr v-for="child in group.children" :key="child.channel" class="call-group call-child">
            <td>{{ child.channel }}</td>
            <td>{{ child.caller }}</td>
            <td>{{ child.callee }}</td>
            <td>{{ formatDuration(child.duration_seconds) }}</td>
            <td>{{ child.codec }}</td>
            <td>{{ child.rtp ? Math.round(child.rtp.rxjitter) + ' ms' : 'N/A' }}</td>
            <td>{{ child.rtp ? Math.round(child.rtp.txjitter) + ' ms' : 'N/A' }}</td>
            <td>{{ child.rtp ? child.rtp.rxploss : 'N/A' }}</td>
            <td>{{ child.rtp ? child.rtp.rtt.toFixed(2) + ' ms' : 'N/A' }}</td>
            <td>
              <template v-if="child.rtp">
                <span class="quality-bar"><span class="fill" :class="mesBarClass(child.rtp.rxmes)" :style="{ width: mesBarWidth(child.rtp.rxmes) }"></span></span>
                {{ child.rtp.rxmes.toFixed(2) }}
              </template>
              <template v-else>N/A</template>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { calls, summary, totalEndpoints, mesClass, mesBarClass, mesBarWidth, formatDuration, formatUptime } from '../composables/useStore'

const trunkPatterns = ['KMG', 'Tronco', 'Flow', 'DAHDI']
const callSearch = ref('')
const callTrunkFilter = ref('')
const callDirectionFilter = ref('')

function isTrunk(name) {
  if (!name) return false
  const upper = name.toUpperCase()
  return trunkPatterns.some(p => upper.includes(p.toUpperCase()))
}

const detectedTrunks = computed(() => {
  const trunks = new Set()
  calls.value.forEach(c => {
    if (isTrunk(c.channel)) trunks.add(c.channel.split('-')[0])
    if (isTrunk(c.linked_channel)) trunks.add(c.linked_channel.split('-')[0])
  })
  return [...trunks].sort()
})

const callTree = computed(() => {
  const byName = {}
  calls.value.forEach(c => { byName[c.channel] = c })
  const groups = {}
  const assigned = new Set()

  calls.value.forEach(c => {
    if (assigned.has(c.channel)) return
    const group = [c]
    assigned.add(c.channel)
    if (c.linked_channel && byName[c.linked_channel] && !assigned.has(c.linked_channel)) {
      group.push(byName[c.linked_channel])
      assigned.add(c.linked_channel)
    }
    if (c.bridge_id) {
      calls.value.forEach(other => {
        if (!assigned.has(other.channel) && other.bridge_id === c.bridge_id) {
          group.push(other)
          assigned.add(other.channel)
        }
      })
    }
    group.sort((a, b) => new Date(a.creation_time) - new Date(b.creation_time))
    const root = group[0]
    groups[root.channel] = { root, children: group.slice(1) }
  })
  return Object.values(groups).sort((a, b) => (b.root.duration_seconds || 0) - (a.root.duration_seconds || 0))
})

const filteredCallTree = computed(() => {
  let groups = callTree.value
  if (callSearch.value) {
    const q = callSearch.value.toLowerCase()
    groups = groups.filter(g => {
      const all = [g.root, ...g.children]
      return all.some(c =>
        (c.channel || '').toLowerCase().includes(q) ||
        (c.caller || '').toLowerCase().includes(q) ||
        (c.callee || '').toLowerCase().includes(q)
      )
    })
  }
  if (callTrunkFilter.value) {
    const t = callTrunkFilter.value
    groups = groups.filter(g => [g.root, ...g.children].some(c =>
      (c.channel || '').startsWith(t) || (c.linked_channel || '').startsWith(t)
    ))
  }
  if (callDirectionFilter.value === 'inbound') {
    groups = groups.filter(g => isTrunk(g.root.channel))
  } else if (callDirectionFilter.value === 'outbound') {
    groups = groups.filter(g => !isTrunk(g.root.channel) && g.children.some(c => isTrunk(c.channel)))
  }
  return groups
})
</script>
