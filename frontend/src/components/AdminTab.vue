<template>
  <div class="admin-panel">
    <div v-if="loading" class="admin-loading">Carregando configura&#231;&#245;es...</div>

    <template v-else>
      <!-- Alertas Gerais -->
      <div class="admin-section">
        <h3>Alertas Gerais</h3>
        <div class="admin-field">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings['alerts.enabled']" />
            <span>Alertas habilitados</span>
          </label>
          <p class="admin-note">Quando habilitado, o sistema envia notifica&#231;&#245;es autom&#225;ticas sobre eventos cr&#237;ticos como queda de qualidade (MOS), endpoints offline e tentativas de acesso n&#227;o autorizadas.</p>
        </div>
      </div>

      <!-- Qualidade de Chamada (MOS) -->
      <div class="admin-section">
        <h3>Qualidade de Chamada (MOS)</h3>
        <div class="admin-field">
          <div class="filter-group">
            <label>Threshold MOS m&#237;nimo</label>
            <input type="number" step="0.1" min="0" max="5" v-model.number="settings['alerts.mos_threshold']" />
          </div>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Cooldown</label>
            <input type="text" v-model="settings['alerts.mos_cooldown']" placeholder="5m" />
          </div>
        </div>
      </div>

      <!-- Endpoint Offline -->
      <div class="admin-section">
        <h3>Endpoint Offline</h3>
        <div class="admin-field">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings['alerts.endpoint_enabled']" />
            <span>Habilitado</span>
          </label>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Cooldown</label>
            <input type="text" v-model="settings['alerts.endpoint_cooldown']" placeholder="5m" />
          </div>
        </div>
      </div>

      <!-- Seguranca (Brute Force) -->
      <div class="admin-section">
        <h3>Seguran&#231;a (Brute Force)</h3>
        <div class="admin-field">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings['alerts.security_enabled']" />
            <span>Habilitado</span>
          </label>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Threshold tentativas</label>
            <input type="number" step="1" min="1" v-model.number="settings['alerts.security_threshold']" />
          </div>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Cooldown</label>
            <input type="text" v-model="settings['alerts.security_cooldown']" placeholder="10m" />
          </div>
        </div>
      </div>

      <!-- Telegram -->
      <div class="admin-section">
        <h3>Telegram</h3>
        <div class="admin-field">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings['telegram.enabled']" />
            <span>Habilitado</span>
          </label>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Bot Token</label>
            <input type="text" v-model="settings['telegram.bot_token']" placeholder="123456:ABC-DEF..." />
          </div>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>Chat ID</label>
            <input type="text" v-model="settings['telegram.chat_id']" placeholder="-1001234567890" />
          </div>
        </div>
        <div class="admin-field">
          <button class="admin-btn admin-btn-secondary" @click="testTelegram" :disabled="testingTelegram">
            {{ testingTelegram ? 'Enviando...' : 'Enviar teste' }}
          </button>
          <span v-if="telegramTestMsg" :class="telegramTestOk ? 'msg-ok' : 'msg-err'">{{ telegramTestMsg }}</span>
        </div>
      </div>

      <!-- Webhook -->
      <div class="admin-section">
        <h3>Webhook</h3>
        <div class="admin-field">
          <label class="toggle-label">
            <input type="checkbox" v-model="settings['webhook.enabled']" />
            <span>Habilitado</span>
          </label>
        </div>
        <div class="admin-field">
          <div class="filter-group">
            <label>URL</label>
            <input type="text" v-model="settings['webhook.url']" placeholder="https://exemplo.com/webhook" class="admin-input-wide" />
          </div>
        </div>
        <div class="admin-field">
          <button class="admin-btn admin-btn-secondary" @click="testWebhook" :disabled="testingWebhook">
            {{ testingWebhook ? 'Enviando...' : 'Enviar teste' }}
          </button>
          <span v-if="webhookTestMsg" :class="webhookTestOk ? 'msg-ok' : 'msg-err'">{{ webhookTestMsg }}</span>
        </div>
      </div>

      <!-- Save -->
      <div class="admin-actions">
        <button class="admin-btn admin-btn-primary" @click="saveSettings" :disabled="saving">
          {{ saving ? 'Salvando...' : 'Salvar Configura\u00e7\u00f5es' }}
        </button>
        <span v-if="saveMsg" :class="saveOk ? 'msg-ok' : 'msg-err'">{{ saveMsg }}</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'

const API = '/api/v1/admin'

const loading = ref(true)
const saving = ref(false)
const saveMsg = ref('')
const saveOk = ref(false)

const testingTelegram = ref(false)
const telegramTestMsg = ref('')
const telegramTestOk = ref(false)

const testingWebhook = ref(false)
const webhookTestMsg = ref('')
const webhookTestOk = ref(false)

const defaults = {
  'alerts.enabled': false,
  'alerts.mos_threshold': 3.0,
  'alerts.mos_cooldown': '5m',
  'alerts.endpoint_enabled': false,
  'alerts.endpoint_cooldown': '5m',
  'alerts.security_enabled': false,
  'alerts.security_threshold': 10,
  'alerts.security_cooldown': '10m',
  'telegram.enabled': false,
  'telegram.bot_token': '',
  'telegram.chat_id': '',
  'webhook.enabled': false,
  'webhook.url': '',
}

const settings = reactive({ ...defaults })

onMounted(async () => {
  try {
    const res = await fetch(API + '/settings')
    if (res.ok) {
      const data = await res.json()
      for (const key of Object.keys(defaults)) {
        if (data[key] !== undefined) {
          settings[key] = data[key]
        }
      }
    }
  } catch (e) {
    console.error('Erro ao carregar configura\u00e7\u00f5es:', e)
  } finally {
    loading.value = false
  }
})

async function saveSettings() {
  saving.value = true
  saveMsg.value = ''
  try {
    const res = await fetch(API + '/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...settings }),
    })
    if (res.ok) {
      saveOk.value = true
      saveMsg.value = 'Configura\u00e7\u00f5es salvas com sucesso!'
    } else {
      saveOk.value = false
      saveMsg.value = 'Erro ao salvar: ' + res.status
    }
  } catch (e) {
    saveOk.value = false
    saveMsg.value = 'Erro de conex\u00e3o ao salvar.'
  } finally {
    saving.value = false
    setTimeout(() => { saveMsg.value = '' }, 5000)
  }
}

async function testTelegram() {
  testingTelegram.value = true
  telegramTestMsg.value = ''
  try {
    const res = await fetch(API + '/test/telegram', { method: 'POST' })
    if (res.ok) {
      telegramTestOk.value = true
      telegramTestMsg.value = 'Mensagem de teste enviada!'
    } else {
      telegramTestOk.value = false
      const data = await res.json().catch(() => ({}))
      telegramTestMsg.value = data.error || 'Erro ao enviar teste: ' + res.status
    }
  } catch (e) {
    telegramTestOk.value = false
    telegramTestMsg.value = 'Erro de conex\u00e3o.'
  } finally {
    testingTelegram.value = false
    setTimeout(() => { telegramTestMsg.value = '' }, 5000)
  }
}

async function testWebhook() {
  testingWebhook.value = true
  webhookTestMsg.value = ''
  try {
    const res = await fetch(API + '/test/webhook', { method: 'POST' })
    if (res.ok) {
      webhookTestOk.value = true
      webhookTestMsg.value = 'Webhook de teste enviado!'
    } else {
      webhookTestOk.value = false
      const data = await res.json().catch(() => ({}))
      webhookTestMsg.value = data.error || 'Erro ao enviar teste: ' + res.status
    }
  } catch (e) {
    webhookTestOk.value = false
    webhookTestMsg.value = 'Erro de conex\u00e3o.'
  } finally {
    testingWebhook.value = false
    setTimeout(() => { webhookTestMsg.value = '' }, 5000)
  }
}
</script>

<style scoped>
.admin-panel {
  max-width: 720px;
}

.admin-loading {
  color: var(--text-muted);
  font-size: 14px;
  padding: 32px 0;
}

.admin-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 16px;
}

.admin-section h3 {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.admin-field {
  margin-bottom: 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.admin-field:last-child {
  margin-bottom: 0;
}

.admin-field .filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.admin-field .filter-group label {
  display: block;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
}

.admin-field .filter-group input {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 7px 12px;
  color: var(--text);
  font-size: 13px;
  width: 220px;
}

.admin-field .filter-group input:focus {
  outline: none;
  border-color: var(--accent);
}

.admin-field .filter-group input.admin-input-wide {
  width: 400px;
}

.admin-note {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
  margin-top: 4px;
}

/* Toggle / checkbox */
.toggle-label {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text);
}

.toggle-label input[type="checkbox"] {
  appearance: none;
  -webkit-appearance: none;
  width: 36px;
  height: 20px;
  background: var(--border);
  border-radius: 10px;
  position: relative;
  cursor: pointer;
  transition: background 0.2s;
  flex-shrink: 0;
}

.toggle-label input[type="checkbox"]::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  background: var(--text-muted);
  border-radius: 50%;
  transition: transform 0.2s, background 0.2s;
}

.toggle-label input[type="checkbox"]:checked {
  background: var(--accent);
}

.toggle-label input[type="checkbox"]:checked::after {
  transform: translateX(16px);
  background: #fff;
}

/* Buttons */
.admin-btn {
  border: none;
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s, background 0.2s;
  font-family: inherit;
}

.admin-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.admin-btn-primary {
  background: var(--accent);
  color: #fff;
}

.admin-btn-primary:hover:not(:disabled) {
  opacity: 0.85;
}

.admin-btn-secondary {
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--text);
}

.admin-btn-secondary:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.admin-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 8px;
  padding-top: 8px;
}

/* Feedback messages */
.msg-ok {
  font-size: 12px;
  color: var(--green);
}

.msg-err {
  font-size: 12px;
  color: var(--red);
}
</style>
