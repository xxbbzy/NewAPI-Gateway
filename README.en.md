<p align="right">
    <a href="./README.md">中文</a> | <strong>English</strong>
</p>

<div align="center">

# NewAPI Gateway

_✨ Multi-provider NewAPI aggregation gateway — unified access, transparent proxy, usage analytics ✨_

</div>

## Overview

NewAPI Gateway is a transparent gateway that aggregates multiple [NewAPI](https://github.com/QuantumNous/new-api) providers. Users access all connected AI model services through a single aggregated token (`ag-xxx`). The system uses **priority-tiered routing with value-aware weighting and optional health adjustment**, and upstream providers cannot detect the gateway's presence.

### Key Features

- ✅ **Transparent Proxy**: Header sanitization, zero body modification, User-Agent passthrough
- ✅ **Multi-Provider Management**: Unified management of tokens, pricing, and balance across NewAPI instances
- ✅ **Smart Routing**: Candidate normalization + priority tiers + value-aware weighted retry + optional health adjustment
- ✅ **Auto Sync**: Syncs pricing/tokens/balance from upstream every 5 minutes, auto-rebuilds route table
- ✅ **Check-in Service**: Automatic daily check-in for enabled providers
- ✅ **SSE Streaming**: Full Server-Sent Events streaming proxy support
- ✅ **Usage Analytics**: Detailed logging of model/provider/latency/status per request
- ✅ **OpenAI Compatible**: Supports OpenAI / Anthropic / Gemini API formats
- ✅ **Web Dashboard**: React frontend with dashboard, provider management, token management, logs

> **This gateway does not handle billing/payments** — it only provides transparent proxying and usage statistics.

---

## Quick Start

### Option 1: DockerHub prebuilt image (recommended)

```bash
docker pull xxbbzy/newapi-gateway:latest
docker run -d --name newapi-gateway \
  --restart always \
  -p 3000:3000 \
  -v ./data:/data \
  xxbbzy/newapi-gateway:latest
```

### Option 2: Start from prebuilt binary

1. Download the binary for your OS/arch from [Releases](https://github.com/xxbbzy/NewAPI-Gateway/releases).
2. Grant execution permission and run:

```bash
chmod +x ./gateway-aggregator
./gateway-aggregator --port 3000 --log-dir ./logs
```

### Option 3: Build from source (kept)

> Best for customization. Requires Go 1.18+, Node.js 16+, and SQLite (default) / MySQL / PostgreSQL.

```bash
# 1. Clone
git clone <repo-url>
cd NewAPI-Gateway-main

# 2. Build frontend
cd web && npm install && npm run build && cd ..

# 3. Build backend and run
go mod download
go build -ldflags "-s -w -X 'NewAPI-Gateway/common.Version=$(cat VERSION)'" -o gateway-aggregator
./gateway-aggregator --port 3000 --log-dir ./logs
```

### First Login

Visit `http://localhost:3000/` — Default credentials: `root` / `123456`

---

## Usage Guide

### 1. Add Provider

Navigate to **Providers** page and click "Add Provider":

| Field           | Description                                | Example                      |
| --------------- | ------------------------------------------ | ---------------------------- |
| Name            | Provider identifier                        | `Provider-A`                 |
| Base URL        | Upstream NewAPI URL                        | `https://api.provider-a.com` |
| Access Token    | Upstream access_token                      | `eyJhbGci...`                |
| User ID         | Upstream user ID (for New-Api-User header) | `1`                          |
| Weight          | Routing weight (higher = more traffic)     | `10`                         |
| Priority        | Routing tier (higher = tried first)        | `0`                          |
| Enable Check-in | Auto daily check-in                        | ☑️                            |

### 2. Sync Data

Click **Sync** on a provider to:
1. Fetch model pricing (`GET /api/pricing`)
2. Fetch sk-Token list (`GET /api/token/`)
3. Fetch user balance (`GET /api/user/self`)
4. Auto-rebuild model routes

### 3. Create Aggregated Token

Go to **Tokens** page and create a token. Supports expiration, model whitelist, and IP whitelist.

### 4. Call API

```bash
curl https://your-gateway.com/v1/chat/completions \
  -H "Authorization: Bearer ag-xxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello!"}]}'
```

---

## API Reference

For complete endpoint and auth details, see [`docs/API_REFERENCE.md`](./docs/API_REFERENCE.md).

### Relay API (OpenAI-compatible, ag-Token auth)

| Method | Path                       | Description               |
| ------ | -------------------------- | ------------------------- |
| POST   | `/v1/chat/completions`     | Chat completions          |
| POST   | `/v1/completions`          | Text completions          |
| POST   | `/v1/embeddings`           | Embeddings                |
| POST   | `/v1/images/generations`   | Image generation          |
| POST   | `/v1/audio/speech`         | Text-to-speech            |
| POST   | `/v1/audio/transcriptions` | Speech-to-text            |
| POST   | `/v1/messages`             | Anthropic Claude compat   |
| POST   | `/v1/responses`            | OpenAI Responses API      |
| POST   | `/v1beta/models/*`         | Gemini compat             |
| GET    | `/v1/models`               | List all available models |

### Management API (Session/Token auth)

| Group     | Method              | Path                        | Description           |
| --------- | ------------------- | --------------------------- | --------------------- |
| Provider  | GET/POST/PUT/DELETE | `/api/provider/`            | Provider CRUD         |
| Provider  | POST                | `/api/provider/:id/sync`    | Trigger sync          |
| Provider  | POST                | `/api/provider/:id/checkin` | Manual check-in       |
| Provider  | POST                | `/api/provider/checkin/run` | Trigger full check-in |
| Provider  | GET                 | `/api/provider/checkin/summary` | Check-in summaries |
| Provider  | GET                 | `/api/provider/checkin/messages` | Check-in messages |
| Provider  | GET                 | `/api/provider/checkin/uncheckin` | Unchecked providers |
| Token     | GET/POST/PUT/DELETE | `/api/agg-token/`           | Aggregated token CRUD |
| Route     | GET                 | `/api/route/`               | View route table      |
| Route     | POST                | `/api/route/rebuild`        | Rebuild routes        |
| Log       | GET                 | `/api/log/self`             | User logs             |
| Log       | GET                 | `/api/log/`                 | All logs (admin)      |
| Dashboard | GET                 | `/api/dashboard`            | Statistics (admin)    |

---

## Configuration

### Environment Variables

| Variable            | Description                                  | Example                                |
| ------------------- | -------------------------------------------- | -------------------------------------- |
| `PORT`              | Listening port                               | `3000`                                 |
| `SQL_DRIVER`        | SQL driver (optional)                        | `sqlite` / `mysql` / `postgres`        |
| `SQL_DSN`           | Database DSN (required for MySQL/PostgreSQL) | `root:pwd@tcp(localhost:3306)/gateway` |
| `REDIS_CONN_STRING` | Redis (for rate-limit & session)             | `redis://default:pw@localhost:6379`    |
| `SESSION_SECRET`    | Fixed session secret                         | `random_string`                        |
| `GIN_MODE`          | Run mode                                     | `release` / `debug`                    |

If `SQL_DRIVER` is not set, backward-compatible behavior is used:
- `SQL_DSN` not set: uses SQLite (`SQLITE_PATH`, default `gateway-aggregator.db`)
- `SQL_DSN` set: auto-detects PostgreSQL (`postgres://`, `postgresql://`, or `dbname=... user=...`), otherwise treats it as MySQL

### Command Line Arguments

| Arg         | Description   | Default |
| ----------- | ------------- | ------- |
| `--port`    | Server port   | `3000`  |
| `--log-dir` | Log directory | none    |
| `--version` | Print version | -       |

---

## Stealth Strategy

The gateway ensures upstream providers **cannot detect its presence**:

| Strategy               | Implementation                             |
| ---------------------- | ------------------------------------------ |
| Auth replacement       | `ag-xxx` → upstream `sk-xxx`               |
| Proxy header removal   | Delete `X-Forwarded-*`, `Via`, `Forwarded` |
| User-Agent passthrough | Preserve original client UA                |
| Zero body modification | Request body forwarded as-is               |
| No custom headers      | No gateway-identifying headers added       |

---

## Documentation Hub

- Docs index: [`docs/README.md`](./docs/README.md)
- Docs architecture: [`docs/DOCS_ARCHITECTURE.md`](./docs/DOCS_ARCHITECTURE.md)
- Quick start: [`docs/QUICK_START.md`](./docs/QUICK_START.md)
- Architecture: [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)
- API reference: [`docs/API_REFERENCE.md`](./docs/API_REFERENCE.md)
- Configuration: [`docs/CONFIGURATION.md`](./docs/CONFIGURATION.md)
- Deployment: [`docs/DEPLOYMENT.md`](./docs/DEPLOYMENT.md)
- Operations: [`docs/OPERATIONS.md`](./docs/OPERATIONS.md)
- Project structure: [`docs/PROJECT_STRUCTURE.md`](./docs/PROJECT_STRUCTURE.md)
- Development guide: [`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
- Database schema: [`docs/DATABASE_SCHEMA.md`](./docs/DATABASE_SCHEMA.md)
- Model alias mapping: [`docs/model-alias-manual-mapping.md`](./docs/model-alias-manual-mapping.md)
- FAQ: [`docs/FAQ.md`](./docs/FAQ.md)

---

## License

MIT License

This project includes third-party code under the MIT License. See `THIRD_PARTY_NOTICES.md` for details.
