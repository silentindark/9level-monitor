<template>
  <div class="sec-cards">
    <div class="sec-card">
      <div class="label">Total de Eventos</div>
      <div class="value" style="color: var(--red)">{{ securitySummary.total_events || 0 }}</div>
    </div>
    <div class="sec-card">
      <div class="label">Senha Inv&#225;lida</div>
      <div class="value" style="color: var(--red)">{{ (securitySummary.events_by_type || {})['InvalidPassword'] || (securitySummary.events_by_type || {})['ChallengeResponseFailed'] || 0 }}</div>
    </div>
    <div class="sec-card">
      <div class="label">Conta Inexistente</div>
      <div class="value" style="color: var(--orange)">{{ (securitySummary.events_by_type || {})['InvalidAccountID'] || 0 }}</div>
    </div>
    <div class="sec-card">
      <div class="label">ACL Bloqueado</div>
      <div class="value" style="color: var(--yellow)">{{ (securitySummary.events_by_type || {})['FailedACL'] || 0 }}</div>
    </div>
    <div class="sec-card">
      <div class="label">IPs &#218;nicos</div>
      <div class="value" style="color: var(--accent)">{{ (securitySummary.top_offenders || []).length }}</div>
    </div>
  </div>

  <!-- Top Offenders -->
  <div v-if="securitySummary.top_offenders && securitySummary.top_offenders.length > 0" style="margin-bottom: 16px">
    <h3 style="font-size: 13px; color: var(--text-muted); margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.5px">Top Ofensores</h3>
    <div class="table-wrap">
      <table>
        <thead><tr><th>IP</th><th>Tentativas</th><th>&#218;ltimo Ramal</th><th>Tipos</th><th>&#218;ltima vez</th></tr></thead>
        <tbody>
          <tr v-for="off in securitySummary.top_offenders" :key="off.remote_address">
            <td style="font-family: 'Courier New', monospace; color: var(--red)">{{ off.remote_address }}</td>
            <td><strong>{{ off.count }}</strong></td>
            <td>{{ off.last_account || '-' }}</td>
            <td>
              <span v-for="t in off.event_types" :key="t" class="badge" :class="secBadgeClass(t)" style="margin-right: 4px">{{ secEventLabel(t) }}</span>
            </td>
            <td>{{ formatTime(off.last_seen) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Events Log -->
  <div class="search">
    <input v-model="securitySearch" placeholder="Buscar IP ou ramal..." />
    <select v-model="securityFilter">
      <option value="all">Todos</option>
      <option value="InvalidPassword">Senha Inv&#225;lida</option>
      <option value="ChallengeResponseFailed">Challenge Falhou</option>
      <option value="InvalidAccountID">Conta Inexistente</option>
      <option value="FailedACL">ACL Bloqueado</option>
      <option value="UnexpectedAddress">IP Inesperado</option>
      <option value="RequestBadFormat">Request Malformado</option>
    </select>
  </div>
  <div v-if="filteredSecurityEvents.length === 0" class="card" style="text-align:center;padding:40px;color:var(--text-muted)">
    Nenhum evento de seguran&#231;a registrado
  </div>
  <div v-else class="table-wrap">
    <table>
      <thead><tr><th>Hor&#225;rio</th><th>Tipo</th><th>Ramal</th><th>IP Origem</th><th>Servi&#231;o</th></tr></thead>
      <tbody>
        <tr v-for="ev in filteredSecurityEvents" :key="ev.id">
          <td style="font-family: 'Courier New', monospace; font-size: 12px">{{ formatTime(ev.timestamp) }}</td>
          <td><span class="badge" :class="secBadgeClass(ev.event_type)">{{ secEventLabel(ev.event_type) }}</span></td>
          <td>{{ ev.account_id || '-' }}</td>
          <td style="font-family: 'Courier New', monospace">{{ ev.remote_address }}</td>
          <td>{{ ev.service || '-' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
  <div class="pagination" v-if="securityPages > 1">
    <button @click="goSecPage(1)" :disabled="securityPage === 1">&laquo;</button>
    <button @click="goSecPage(securityPage - 1)" :disabled="securityPage === 1">&lsaquo; Anterior</button>
    <span class="page-info">{{ securityPage }} / {{ securityPages }} ({{ securityTotal }} eventos)</span>
    <button @click="goSecPage(securityPage + 1)" :disabled="securityPage === securityPages">Pr&#243;ximo &rsaquo;</button>
    <button @click="goSecPage(securityPages)" :disabled="securityPage === securityPages">&raquo;</button>
  </div>
  <div v-else-if="securityTotal > 0" style="text-align:center;margin-top:12px;font-size:12px;color:var(--text-muted)">
    {{ securityTotal }} eventos (expurgo autom&#225;tico a cada 12h)
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import {
  securityEvents, securitySummary, securityPage, securityTotal, securityPages,
  fetchSecurityPage, formatTime, secEventLabel, secBadgeClass
} from '../composables/useStore'

const securitySearch = ref('')
const securityFilter = ref('all')

const filteredSecurityEvents = computed(() => {
  let list = securityEvents.value
  if (securityFilter.value !== 'all') {
    list = list.filter(ev => ev.event_type === securityFilter.value)
  }
  if (securitySearch.value) {
    const q = securitySearch.value.toLowerCase()
    list = list.filter(ev =>
      (ev.account_id && ev.account_id.toLowerCase().includes(q)) ||
      (ev.remote_address && ev.remote_address.toLowerCase().includes(q))
    )
  }
  return list
})

function goSecPage(p) {
  if (p < 1 || p > securityPages.value) return
  fetchSecurityPage(p)
}
</script>
