<template>
  <div class="filters-bar">
    <div class="filter-group">
      <label>Per&#237;odo</label>
      <select v-model="chartPeriod" @change="onPeriodChange()">
        <option value="today">Hoje</option>
        <option value="yesterday">Ontem</option>
        <option value="7d">&#218;ltimos 7 dias</option>
        <option value="30d">&#218;ltimos 30 dias</option>
        <option value="custom">Personalizado</option>
      </select>
    </div>
    <template v-if="chartPeriod === 'custom'">
      <div class="filter-group">
        <label>De</label>
        <input type="date" v-model="chartFrom" @change="loadCharts()">
      </div>
      <div class="filter-group">
        <label>At&#233;</label>
        <input type="date" v-model="chartTo" @change="loadCharts()">
      </div>
    </template>
  </div>
  <div class="charts-grid">
    <div class="chart-box">
      <h3>Chamadas por Hora{{ isMultiDay ? ' (m\u00e9dia no per\u00edodo)' : '' }}</h3>
      <canvas ref="canvasCalls" height="200"></canvas>
    </div>
    <div class="chart-box">
      <h3>MOS M&#233;dio por Hora</h3>
      <canvas ref="canvasMos" height="200"></canvas>
    </div>
    <div class="chart-box">
      <h3>Chamadas com MOS &lt; 3.0 por Hora</h3>
      <canvas ref="canvasBad" height="200"></canvas>
    </div>
    <div class="chart-box" v-if="isMultiDay">
      <h3>Chamadas por Dia</h3>
      <canvas ref="canvasDaily" height="200"></canvas>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

const BASE = import.meta.env.BASE_URL.replace(/\/$/, '')

function todayStr() { return new Date().toISOString().slice(0, 10) }
function yesterdayStr() { const d = new Date(); d.setDate(d.getDate() - 1); return d.toISOString().slice(0, 10) }
function daysAgoStr(n) { const d = new Date(); d.setDate(d.getDate() - n); return d.toISOString().slice(0, 10) }

const chartPeriod = ref('today')
const chartFrom = ref(todayStr())
const chartTo = ref(todayStr())
const canvasCalls = ref(null)
const canvasMos = ref(null)
const canvasBad = ref(null)
const canvasDaily = ref(null)
let instances = {}

const isMultiDay = computed(() => chartFrom.value !== chartTo.value)

onMounted(() => { loadCharts() })
onUnmounted(() => { Object.values(instances).forEach(c => c.destroy()) })

defineExpose({ loadCharts })

function onPeriodChange() {
  switch (chartPeriod.value) {
    case 'today':
      chartFrom.value = todayStr()
      chartTo.value = todayStr()
      break
    case 'yesterday':
      chartFrom.value = yesterdayStr()
      chartTo.value = yesterdayStr()
      break
    case '7d':
      chartFrom.value = daysAgoStr(6)
      chartTo.value = todayStr()
      break
    case '30d':
      chartFrom.value = daysAgoStr(29)
      chartTo.value = todayStr()
      break
  }
  loadCharts()
}

async function loadCharts() {
  await nextTick()
  try {
    const params = 'from=' + chartFrom.value + '&to=' + chartTo.value

    const [hourlyData, dailyData] = await Promise.all([
      fetch(BASE + '/api/v1/history/calls/hourly?' + params).then(r => r.ok ? r.json() : []),
      isMultiDay.value
        ? fetch(BASE + '/api/v1/history/calls/daily?' + params).then(r => r.ok ? r.json() : [])
        : Promise.resolve(null),
    ])

    const labels = hourlyData.map(d => d.hour + 'h')
    const totalCalls = hourlyData.map(d => d.total_calls)
    const avgMos = hourlyData.map(d => d.avg_mos)
    const badCalls = hourlyData.map(d => d.bad_calls)

    Object.values(instances).forEach(c => c.destroy())
    instances = {}

    const gridColor = 'rgba(42, 45, 58, 0.8)'
    const tickColor = '#8b8fa3'

    if (canvasCalls.value) {
      instances.calls = new Chart(canvasCalls.value.getContext('2d'), {
        type: 'bar',
        data: { labels, datasets: [{ label: 'Chamadas', data: totalCalls, backgroundColor: '#4f8cff' }] },
        options: { responsive: true, plugins: { legend: { labels: { color: tickColor } } }, scales: { y: { beginAtZero: true, ticks: { color: tickColor }, grid: { color: gridColor } }, x: { ticks: { color: tickColor }, grid: { color: gridColor } } } }
      })
    }

    if (canvasMos.value) {
      instances.mos = new Chart(canvasMos.value.getContext('2d'), {
        type: 'line',
        data: {
          labels,
          datasets: [
            { label: 'MOS M\u00e9dio', data: avgMos, borderColor: '#34d399', backgroundColor: 'rgba(52,211,153,0.1)', fill: true, tension: 0.3 },
            { label: 'Limite', data: Array(24).fill(3.0), borderColor: '#f87171', borderDash: [5, 5], pointRadius: 0, fill: false }
          ]
        },
        options: { responsive: true, plugins: { legend: { labels: { color: tickColor } } }, scales: { y: { min: 0, max: 5, ticks: { color: tickColor }, grid: { color: gridColor } }, x: { ticks: { color: tickColor }, grid: { color: gridColor } } } }
      })
    }

    if (canvasBad.value) {
      instances.bad = new Chart(canvasBad.value.getContext('2d'), {
        type: 'bar',
        data: { labels, datasets: [{ label: 'MOS < 3.0', data: badCalls, backgroundColor: '#f87171' }] },
        options: { responsive: true, plugins: { legend: { labels: { color: tickColor } } }, scales: { y: { beginAtZero: true, ticks: { color: tickColor }, grid: { color: gridColor } }, x: { ticks: { color: tickColor }, grid: { color: gridColor } } } }
      })
    }

    // Daily chart (multi-day only)
    if (dailyData && dailyData.length > 0) {
      await nextTick()
      if (canvasDaily.value) {
        const dayLabels = dailyData.map(d => {
          const parts = d.date.split('-')
          return parts[2] + '/' + parts[1]
        })
        const dayCalls = dailyData.map(d => d.total_calls)
        const dayMos = dailyData.map(d => d.avg_mos)

        instances.daily = new Chart(canvasDaily.value.getContext('2d'), {
          type: 'bar',
          data: {
            labels: dayLabels,
            datasets: [
              { label: 'Chamadas', data: dayCalls, backgroundColor: '#4f8cff', yAxisID: 'y' },
              { label: 'MOS M\u00e9dio', data: dayMos, type: 'line', borderColor: '#34d399', backgroundColor: 'transparent', yAxisID: 'y1', tension: 0.3, pointRadius: 3 }
            ]
          },
          options: {
            responsive: true,
            plugins: { legend: { labels: { color: tickColor } } },
            scales: {
              y: { beginAtZero: true, position: 'left', ticks: { color: tickColor }, grid: { color: gridColor }, title: { display: true, text: 'Chamadas', color: tickColor } },
              y1: { min: 0, max: 5, position: 'right', grid: { drawOnChartArea: false }, ticks: { color: '#34d399' }, title: { display: true, text: 'MOS', color: '#34d399' } },
              x: { ticks: { color: tickColor }, grid: { color: gridColor } }
            }
          }
        })
      }
    }
  } catch (e) { console.error('chart error:', e) }
}
</script>
