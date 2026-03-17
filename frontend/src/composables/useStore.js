import { ref, computed, reactive } from 'vue'

const API = '/api'

// Shared reactive state
export const connected = ref(false)
export const lastUpdate = ref('')
export const calls = ref([])
export const endpoints = ref([])
export const summary = ref({})
export const health = ref({})
export const dbSize = ref('')
export const securityEvents = ref([])
export const securitySummary = reactive({ total_events: 0, events_by_type: {}, top_offenders: [] })
export const securityPage = ref(1)
export const securityTotal = ref(0)
export const securityPages = ref(1)
export const securityPerPage = 50

export const totalEndpoints = computed(() => endpoints.value.length)

// --- Helpers ---
export function latencyClass(ms) {
  if (ms == null) return 'cyber-blue'
  if (ms < 5) return 'cyber-green'
  if (ms < 20) return 'cyber-blue'
  if (ms < 50) return 'cyber-yellow'
  if (ms < 100) return 'cyber-warn'
  return 'cyber-crit'
}
export function rtpPollClass(ms) {
  if (ms == null) return 'cyber-blue'
  if (ms < 500) return 'cyber-green'
  if (ms < 2000) return 'cyber-blue'
  if (ms < 5000) return 'cyber-yellow'
  return 'cyber-crit'
}
export function queueClass(len) {
  if (len == null) return 'cyber-blue'
  if (len < 10) return 'cyber-green'
  if (len < 50) return 'cyber-yellow'
  if (len < 128) return 'cyber-warn'
  return 'cyber-crit'
}
export function mesClass(val) {
  if (!val || val === 0) return ''
  if (val >= 4.0) return 'green'
  if (val >= 3.0) return 'yellow'
  return 'accent'
}
export function mesBarClass(val) {
  if (val >= 4.0) return 'q-good'
  if (val >= 3.0) return 'q-warn'
  return 'q-bad'
}
export function mesBarWidth(val) {
  return Math.min(100, Math.max(0, (val / 5) * 100)) + '%'
}
export function formatDuration(seconds) {
  if (!seconds) return '0s'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}h${m}m${s}s`
  if (m > 0) return `${m}m${s}s`
  return `${s}s`
}
export function formatUptime(seconds) {
  if (!seconds) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
export function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
export function formatDateTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export const secEventLabels = {
  'InvalidPassword': 'Senha Inv\u00e1lida',
  'ChallengeResponseFailed': 'Challenge Falhou',
  'InvalidAccountID': 'Conta Inexistente',
  'FailedACL': 'ACL Bloqueado',
  'UnexpectedAddress': 'IP Inesperado',
  'RequestBadFormat': 'Malformado',
}
export function secEventLabel(type) { return secEventLabels[type] || type }
export function secBadgeClass(type) {
  if (['InvalidPassword', 'ChallengeResponseFailed', 'FailedACL', 'UnexpectedAddress'].includes(type)) return 'danger'
  return 'warning'
}

// --- Initial fetch (one-time, before SSE connects) ---
export async function initialFetch() {
  try {
    const [monRes, secEvRes, secSumRes] = await Promise.all([
      fetch(API + '/monitor'),
      fetch(API + '/v1/security?page=1&per_page=' + securityPerPage),
      fetch(API + '/v1/security/summary'),
    ])
    if (monRes.ok) {
      const data = await monRes.json()
      calls.value = data.calls || []
      endpoints.value = data.endpoints || []
      summary.value = data.summary || {}
    }
    if (secEvRes.ok) {
      const data = await secEvRes.json()
      securityEvents.value = data.events || []
      securityTotal.value = data.total || 0
      securityPages.value = data.pages || 1
    }
    if (secSumRes.ok) Object.assign(securitySummary, await secSumRes.json())
    lastUpdate.value = new Date().toLocaleTimeString('pt-BR')
  } catch (e) { /* ignore */ }
}

// Security pagination (on-demand, not polled)
export async function fetchSecurityPage(page) {
  if (page) securityPage.value = page
  try {
    const res = await fetch(API + '/v1/security?page=' + securityPage.value + '&per_page=' + securityPerPage)
    if (res.ok) {
      const data = await res.json()
      securityEvents.value = data.events || []
      securityTotal.value = data.total || 0
      securityPages.value = data.pages || 1
    }
  } catch (e) { /* ignore */ }
}

// --- SSE: single persistent connection for ALL real-time data ---
let eventSource = null

export function connectSSE() {
  eventSource = new EventSource(API + '/events')

  eventSource.onopen = () => { connected.value = true }

  eventSource.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      lastUpdate.value = new Date().toLocaleTimeString('pt-BR')
      connected.value = true

      if (msg.type && msg.data) {
        switch (msg.type) {
          case 'summary:update':
            summary.value = msg.data
            break
          case 'call:new':
          case 'call:update': {
            const idx = calls.value.findIndex(c => c.channel === msg.data.channel)
            if (idx >= 0) calls.value[idx] = msg.data
            else calls.value.push(msg.data)
            break
          }
          case 'call:end': {
            const idx = calls.value.findIndex(c => c.channel === msg.data.channel)
            if (idx >= 0) calls.value.splice(idx, 1)
            break
          }
          case 'endpoint:update': {
            if (msg.data.contacts && !Array.isArray(msg.data.contacts)) {
              msg.data.contacts = Object.values(msg.data.contacts)
            }
            const idx = endpoints.value.findIndex(ep => ep.name === msg.data.name)
            if (idx >= 0) endpoints.value[idx] = msg.data
            else endpoints.value.push(msg.data)
            break
          }
          case 'health:update':
            health.value = { ...health.value, ...msg.data }
            if (msg.data.db_size) dbSize.value = msg.data.db_size
            break
          case 'security:event':
            if (securityPage.value === 1) {
              securityEvents.value.unshift(msg.data)
              if (securityEvents.value.length > securityPerPage) securityEvents.value.pop()
              securityTotal.value++
            }
            break
          case 'security:events':
            if (securityPage.value === 1 && msg.data.events) {
              msg.data.events.forEach(ev => {
                securityEvents.value.unshift(ev)
                securityTotal.value++
              })
              while (securityEvents.value.length > securityPerPage) securityEvents.value.pop()
            }
            break
          case 'security:summary':
            Object.assign(securitySummary, msg.data)
            break
        }
        return
      }
      // Fallback full payload
      if (msg.calls) calls.value = msg.calls
      if (msg.endpoints) endpoints.value = msg.endpoints
      if (msg.summary) summary.value = msg.summary
    } catch (e) { /* ignore */ }
  }

  eventSource.onerror = () => {
    connected.value = false
    eventSource.close()
    setTimeout(connectSSE, 3000)
  }
}

export function disconnectSSE() {
  if (eventSource) eventSource.close()
}
