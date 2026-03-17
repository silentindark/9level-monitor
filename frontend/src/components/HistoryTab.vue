<template>
  <div class="filters-bar">
    <div class="filter-group">
      <label>Per&#237;odo</label>
      <select v-model="historyPeriod" @change="onPeriodChange()">
        <option value="today">Hoje</option>
        <option value="yesterday">Ontem</option>
        <option value="7d">&#218;ltimos 7 dias</option>
        <option value="30d">&#218;ltimos 30 dias</option>
        <option value="custom">Personalizado</option>
      </select>
    </div>
    <template v-if="historyPeriod === 'custom'">
      <div class="filter-group">
        <label>De</label>
        <input type="date" v-model="historyFrom" @change="loadHistoryCalls()">
      </div>
      <div class="filter-group">
        <label>At&#233;</label>
        <input type="date" v-model="historyTo" @change="loadHistoryCalls()">
      </div>
    </template>
    <div class="filter-group">
      <label>MOS M&#237;nimo</label>
      <input type="number" v-model="historyMinMOS" step="0.1" min="0" max="5" style="width:80px" @change="loadHistoryCalls()">
    </div>
  </div>

  <div class="cards" v-if="historyStats" style="margin-bottom:16px">
    <div class="card">
      <div class="label">Total de Chamadas</div>
      <div class="value accent">{{ historyStats.total_calls }}</div>
    </div>
    <div class="card">
      <div class="label">MOS M&#233;dio</div>
      <div class="value" :class="mesClass(historyStats.avg_mos)">{{ historyStats.avg_mos.toFixed(2) }}</div>
    </div>
    <div class="card">
      <div class="label">Chamadas MOS &lt; 3.0</div>
      <div class="value" style="color:var(--red)">{{ historyStats.calls_below_3 }}</div>
    </div>
  </div>

  <div style="font-size:11px;color:var(--text-muted);margin-bottom:12px;letter-spacing:0.3px">
    Chamadas com dura&#231;&#227;o inferior a 7 segundos n&#227;o s&#227;o armazenadas (sem conversa efetiva).
  </div>

  <div v-if="historyCalls.length === 0" class="card" style="text-align:center;padding:40px;color:var(--text-muted)">
    Nenhuma chamada no per&#237;odo
  </div>
  <div v-else class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>T&#233;rmino</th><th>Canal</th><th>Origem</th><th>Destino</th><th>Dura&#231;&#227;o</th>
          <th>MOS Rx</th><th>MOS Tx</th><th>Jitter Rx</th><th>RTT</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in historyCalls" :key="c.id">
          <td style="font-family:'Courier New',monospace;font-size:12px">{{ formatDateTime(c.ended_at) }}</td>
          <td>{{ c.channel }}</td>
          <td>{{ c.caller }}</td>
          <td>{{ c.callee }}</td>
          <td>{{ formatDuration(c.duration_seconds) }}</td>
          <td>
            <span class="quality-bar"><span class="fill" :class="mesBarClass(c.rxmes)" :style="{ width: mesBarWidth(c.rxmes) }"></span></span>
            {{ c.rxmes.toFixed(2) }}
          </td>
          <td>
            <span class="quality-bar"><span class="fill" :class="mesBarClass(c.txmes)" :style="{ width: mesBarWidth(c.txmes) }"></span></span>
            {{ c.txmes.toFixed(2) }}
          </td>
          <td>{{ c.rxjitter.toFixed(1) }} ms</td>
          <td>{{ c.rtt.toFixed(1) }} ms</td>
        </tr>
      </tbody>
    </table>
  </div>
  <div class="pagination" v-if="historyPages > 1">
    <button @click="historyPage=1;loadHistoryCalls()" :disabled="historyPage===1">&laquo;</button>
    <button @click="historyPage--;loadHistoryCalls()" :disabled="historyPage===1">&lsaquo; Anterior</button>
    <span class="page-info">{{ historyPage }} / {{ historyPages }}</span>
    <button @click="historyPage++;loadHistoryCalls()" :disabled="historyPage>=historyPages">Pr&#243;ximo &rsaquo;</button>
    <button @click="historyPage=historyPages;loadHistoryCalls()" :disabled="historyPage>=historyPages">&raquo;</button>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { mesClass, mesBarClass, mesBarWidth, formatDuration, formatDateTime } from '../composables/useStore'

const BASE = import.meta.env.BASE_URL.replace(/\/$/, '')

function todayStr() { return new Date().toISOString().slice(0, 10) }
function yesterdayStr() { const d = new Date(); d.setDate(d.getDate() - 1); return d.toISOString().slice(0, 10) }
function daysAgoStr(n) { const d = new Date(); d.setDate(d.getDate() - n); return d.toISOString().slice(0, 10) }

const historyPeriod = ref('today')
const historyFrom = ref(todayStr())
const historyTo = ref(todayStr())
const historyMinMOS = ref(0)
const historyPage = ref(1)
const historyPages = ref(1)
const historyCalls = ref([])
const historyStats = ref(null)

onMounted(() => { loadHistoryCalls() })

defineExpose({ loadHistoryCalls })

function onPeriodChange() {
  switch (historyPeriod.value) {
    case 'today':
      historyFrom.value = todayStr()
      historyTo.value = todayStr()
      break
    case 'yesterday':
      historyFrom.value = yesterdayStr()
      historyTo.value = yesterdayStr()
      break
    case '7d':
      historyFrom.value = daysAgoStr(6)
      historyTo.value = todayStr()
      break
    case '30d':
      historyFrom.value = daysAgoStr(29)
      historyTo.value = todayStr()
      break
  }
  historyPage.value = 1
  loadHistoryCalls()
}

async function loadHistoryCalls() {
  try {
    const params = new URLSearchParams({
      from: historyFrom.value,
      to: historyTo.value,
      page: historyPage.value,
      per_page: 50,
    })
    if (historyMinMOS.value > 0) params.set('min_mos', historyMinMOS.value)

    const [callsRes, statsRes] = await Promise.all([
      fetch(BASE + '/api/v1/history/calls?' + params).then(r => r.json()),
      fetch(BASE + '/api/v1/history/calls/stats?from=' + historyFrom.value + '&to=' + historyTo.value).then(r => r.json()),
    ])
    historyCalls.value = callsRes.items || []
    historyPages.value = callsRes.pages || 1
    historyStats.value = statsRes
  } catch (e) { console.error('history error:', e) }
}
</script>
