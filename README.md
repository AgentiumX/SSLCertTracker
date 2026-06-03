# SSL Certificate Tracker

A self-hosted SSL certificate monitoring system with Server + Agent architecture, supporting multi-region checking from multiple Agent machines.

## Architecture

- **Server**: Single Go binary with embedded Vue 3 dashboard. Provides Agent API, public Dashboard API, and Admin API.
- **Agent**: Single Go binary deployed across regions. Pulls domain list from Server, performs TLS handshakes, reports results.
- **Database**: SQLite (default) or MySQL via GORM.

## Build

```bash
# Build server (includes frontend build)
make build-server

# Build agent
make build-agent

# Build everything
make build-all
```

The frontend lives in `web/` (Vue 3 + Vite + TypeScript + Tailwind). `make build-web` runs `vite build` and outputs to `server/internal/web/dist/`, which is then `embed`ed into the server binary.

For frontend-only iteration:
```bash
make dev-server  # in one terminal — Go server on :8080
make dev-web     # in another — Vite dev server on :5173 with /api proxy
```

## Run

### Server

```bash
cd server
cp config.yaml.example config.yaml
# Edit config.yaml: set auth.agent_token to a strong secret
./server -config config.yaml
```

Default listen: `:8080`.

### Agent

```bash
cd agent
cp config.yaml.example config.yaml
# Edit config.yaml:
#   - server_url: where Server is reachable
#   - auth_token: must match server's auth.agent_token
#   - agent.display_name: e.g. "Beijing-prod-01"
./agent -config config.yaml
```

## Test

```bash
cd server && go test ./...
cd agent && go test ./...
```

The `agent/e2e` package runs full end-to-end tests: it launches the server binary, registers an Agent over HTTP, executes TLS checks, and verifies results land in the database.

## Admin API

All admin endpoints are at `/api/admin/*`. **Authentication is not yet enforced** in this MVP — running the server on a public network without a reverse-proxy ACL will leak control.

### Create domain

```bash
curl -X POST http://localhost:8080/api/admin/domains \
  -H "Content-Type: application/json" \
  -d '{
    "host": "example.com",
    "port": 443,
    "protocol": "https",
    "is_global": true,
    "remark": "marketing site"
  }'
```

Response: `{"id": 1}`

### List domains

```bash
curl http://localhost:8080/api/admin/domains
```

### Get domain

```bash
curl http://localhost:8080/api/admin/domains/1
```

### Delete domain

```bash
curl -X DELETE http://localhost:8080/api/admin/domains/1
```

## Dashboard

Open `http://localhost:8080/` in a browser to see the public dashboard:

- **Overview** (`/`): 4 stat cards (total domains, healthy, alert, agents online) + a list of all domains showing per-domain health (`X / Y healthy`)
- **Domain detail** (`/domains/:id`): per-Agent result cards showing status, expiry date, issuer, SANs, and any error message

The dashboard is **read-only and unauthenticated** in this release (Plan 3 will add admin login + write operations). Pages auto-refresh every 30 seconds.

## Admin API

All admin endpoints are at `/api/admin/*`. **Authentication is not yet enforced** in this MVP — running the server on a public network without a reverse-proxy ACL will leak control.

### Create domain

```bash
curl -X POST http://localhost:8080/api/admin/domains \
  -H "Content-Type: application/json" \
  -d '{
    "host": "example.com",
    "port": 443,
    "protocol": "https",
    "is_global": true,
    "remark": "marketing site"
  }'
```

Response: `{"id": 1}`

### List domains

```bash
curl http://localhost:8080/api/admin/domains
```

### Get domain

```bash
curl http://localhost:8080/api/admin/domains/1
```

### Delete domain

```bash
curl -X DELETE http://localhost:8080/api/admin/domains/1
```

## Dashboard

Open `http://localhost:8080/` in a browser to see the public dashboard:

- **Overview** (`/`): 4 stat cards (total domains, healthy, alert, agents online) + a list of all domains showing per-domain health (`X / Y healthy`)
- **Domain detail** (`/domains/:id`): per-Agent result cards showing status, expiry date, issuer, SANs, and any error message

The dashboard is **read-only and unauthenticated** in this release (Plan 3 will add admin login + write operations). Pages auto-refresh every 30 seconds.

## Agent API

Agent endpoints require `Authorization: Bearer <agent_token>`.

- `POST /api/agent/register` — first-time registration or display name change
- `GET /api/agent/domains?agent_id=...` — pull domains to check, also serves as heartbeat
- `POST /api/agent/results` — batch report TLS check results

## Status semantics

- Agent reports: `ok | expired | mismatch | unreachable`
- Server reclassifies `ok` to `expiring` when `not_after - now < expire_threshold_days`

## Data model

See [SSL Cert Tracker design spec](docs/superpowers/specs/2026-06-02-ssl-cert-tracker-design.md) for full schema and architecture details.

## Roadmap

- **Plan 3**: Alert engine (email, webhook, DingTalk, Feishu, WeCom) + production hardening (admin auth, history retention, daily reminders)
