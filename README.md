# 9Level Monitor

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Real-time Asterisk PBX monitoring collector. Connects to Asterisk's native AMI (TCP) and ARI (HTTP) interfaces to capture call events, RTP quality metrics, and PJSIP endpoint status with minimal latency.

## Features

- **Real-time call tracking** via AMI events (Newchannel, Hangup, DialBegin, BridgeEnter/Leave)
- **RTP quality metrics** — MOS, jitter, packet loss, RTT per channel (via RTCP events + ARI polling)
- **PJSIP endpoint monitoring** — online/offline state, contact registration, qualify RTT
- **Security event detection** — failed auth attempts, ACL violations, unexpected addresses
- **SSE streaming** — push updates to frontend clients in real-time
- **Historical data** — SQLite persistence for call quality, endpoint changes, and security events
- **REST API** — full JSON API for integration with dashboards and alerting systems

## Quick Start

```bash
# Clone and configure
cp .env.example .env
# Edit .env with your Asterisk AMI/ARI credentials

# Build and run
go build -o collector ./cmd/collector
./collector

# Or with Docker
docker compose up -d
```

The collector listens on port 3001 by default.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    9level-monitor (Go)                            │
│                        Port 3001                                 │
│                                                                  │
│  ┌────────────┐  ┌──────────┐  ┌───────┐  ┌──────────────────┐  │
│  │  Collector  │  │   API    │  │  SSE  │  │  Store           │  │
│  │ (event loop)│  │ Handlers │  │Broker │  │  (in-memory)     │  │
│  └─────┬──────┘  └──────────┘  └───────┘  │  - Channels      │  │
│        │                                   │  - Endpoints     │  │
│        │                                   │  - Bridges       │  │
│        │                                   └──────────────────┘  │
└────────┼─────────────────────────────────────────────────────────┘
         │
         │  ┌──── AMI (TCP 5038) ──── Real-time events
         │  │     Newchannel, Hangup, DialBegin, BridgeEnter,
         │  │     BridgeLeave, RTCPSent, RTCPReceived,
         │  │     ContactStatus, PeerStatus, EndpointList
         ├──┤
         │  └──── ARI (HTTP 8088) ──── Periodic polling
         │        GET /channels (bootstrap)
         │        GET /channels/{id}/rtp_statistics
         │        GET /asterisk/info (health check)
         ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Asterisk PBX                               │
│                AMI port 5038  |  ARI port 8088                   │
└──────────────────────────────────────────────────────────────────┘
```

## Configuration

All settings are configured via environment variables (or `.env` file). See [.env.example](.env.example) for a template.

| Variable | Default | Description |
|----------|---------|-------------|
| `AMI_HOST` | `127.0.0.1` | Asterisk AMI host |
| `AMI_PORT` | `5038` | AMI TCP port |
| `AMI_USER` | `9level` | AMI username |
| `AMI_SECRET` | *(required)* | AMI password |
| `ARI_BASE_URL` | `http://127.0.0.1:8088/ari` | ARI REST base URL |
| `ARI_USER` | `9level` | ARI username |
| `ARI_PASS` | *(required)* | ARI password |
| `PORT` | `3001` | HTTP server port |
| `DB_PATH` | `/data/9level.db` | SQLite database path |
| `RTP_POLL_INTERVAL` | `30s` | RTP stats polling interval via ARI |
| `ENDPOINT_REFRESH_INTERVAL` | `5m` | Endpoint re-sync interval via AMI |
| `SECURITY_WHITELIST_IPS` | *(empty)* | Comma-separated IPs to ignore in security events |

## REST API

Base URL: `http://localhost:3001`

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/monitor` | Full view: calls + endpoints + summary |
| `GET /api/v1/calls` | Active calls with RTP metrics |
| `GET /api/v1/calls/{id}` | Single call details |
| `GET /api/v1/endpoints` | All PJSIP endpoints with contacts |
| `GET /api/v1/summary` | Summary: active calls, online endpoints, avg MOS, peak, uptime |
| `GET /api/v1/health` | Health status: AMI/ARI connection, counts, SSE clients |
| `GET /api/v1/events` | SSE real-time event stream |
| `GET /api/v1/security` | Security events (paginated) |
| `GET /api/v1/history/calls` | Historical call quality (SQLite) |
| `GET /api/v1/history/calls/stats` | Aggregated call statistics |
| `GET /api/v1/history/calls/hourly` | Hourly call distribution |
| `GET /api/v1/history/calls/daily` | Daily call totals and avg MOS |
| `GET /api/v1/history/security` | Historical security events |
| `GET /api/v1/history/endpoints` | Endpoint state change history |

### SSE Events

Connect to `/api/v1/events` for real-time updates:

| Event | Description |
|-------|-------------|
| `call:new` | New channel created |
| `call:update` | RTP quality metrics updated |
| `call:end` | Call ended (includes duration) |
| `endpoint:update` | Endpoint state or contact changed |
| `endpoint:state_change` | Endpoint went online/offline |
| `summary:update` | Aggregated summary refresh |
| `health:update` | System health metrics |
| `security:event` | New security event detected |

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

[MIT](LICENSE)
