<template>
  <div class="header">
    <h1>9LEVEL MONITOR</h1>

    <div class="telemetry">
      <div class="telem-item">
        <span class="telem-label">AMI</span>
        <span class="telem-value" :class="latencyClass(health.ami_ms)">{{ health.ami_ms != null ? health.ami_ms.toFixed(1) + 'ms' : '--' }}</span>
        <span class="tooltip">Lat&#234;ncia do Asterisk Manager Interface</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">ARI</span>
        <span class="telem-value" :class="latencyClass(health.ari_ms)">{{ health.ari_ms != null ? health.ari_ms.toFixed(1) + 'ms' : '--' }}</span>
        <span class="tooltip">Lat&#234;ncia do Asterisk REST Interface</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">RTP POLL</span>
        <span class="telem-value" :class="rtpPollClass(health.rtp_poll_ms)">{{ health.rtp_poll_ms != null ? health.rtp_poll_ms.toFixed(0) + 'ms' : '--' }}</span>
        <span class="tooltip">Tempo total da coleta de m&#233;tricas RTP via ARI</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">EVT/s</span>
        <span class="telem-value cyber-blue">{{ health.events_per_sec != null ? health.events_per_sec.toFixed(1) : '--' }}</span>
        <span class="tooltip">Eventos AMI processados por segundo</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">QUEUE</span>
        <span class="telem-value" :class="queueClass(health.ami_queue_len)">{{ health.ami_queue_len != null ? health.ami_queue_len : '--' }}</span>
        <span class="tooltip">Fila de eventos AMI aguardando processamento</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">SSE</span>
        <span class="telem-value cyber-blue">{{ health.sse != null ? health.sse : '--' }}</span>
        <span class="tooltip">Clientes conectados via Server-Sent Events (tempo real)</span>
      </div>
      <div class="telem-sep"></div>
      <div class="telem-item">
        <span class="telem-label">DB</span>
        <span class="telem-value db-badge">{{ dbSize || '--' }}</span>
        <span class="tooltip">Tamanho do banco SQLite (hist&#243;rico 30 dias)</span>
      </div>
    </div>

    <div class="status">
      <div class="dot" :class="{ offline: !connected }"></div>
      <span>{{ connected ? 'ONLINE' : 'OFFLINE' }}</span>
      <span v-if="lastUpdate" style="margin-left: 8px; font-size: 10px; opacity: 0.5">{{ lastUpdate }}</span>
    </div>
  </div>
</template>

<script setup>
import { health, dbSize, connected, lastUpdate, latencyClass, rtpPollClass, queueClass } from '../composables/useStore'
</script>
