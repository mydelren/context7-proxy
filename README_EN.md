# Context7 API Proxy Manager

[简体中文](./README.md) | English

A unified Context7 API gateway for teams and multi-agent setups. Centralize key management, monitor usage, and control access — all from one endpoint.

---

## Why?

When multiple team members or AI agents (Claude Code, Cursor, VS Code, etc.) need access to Context7 documentation, managing individual API keys creates friction:

- **Scattered keys** — everyone requests and stores their own keys
- **No visibility** — who's using what, how much quota is left, no global view
- **Rotation pain** — swapping a key means updating everyone's config
- **No access control** — can't revoke one person's access without affecting others

This project solves all of that: team members only need one endpoint and one Master Key. The admin manages all API key lifecycles centrally.

---

## Features

- **Unified Endpoint** — all agents access Context7 API through one proxy address. No need to distribute real API keys.
- **Master Key Auth** — access via `Authorization: Bearer <MasterKey>`. Admin can reset anytime.
- **Smart Key Pool** — automatically distributes requests across multiple keys for full quota utilization. Transparent failover on rate limits.
- **Web Management UI**:
  - **Dashboard**: stat cards + 24h request chart for team-wide visibility.
  - **Keys**: add, delete, enable/disable API keys.
  - **Logs**: detailed request logs with status code filter.
  - **Settings**: view/reset Master Key, auto-generated MCP client config.
- **i18n & Themes** — Chinese/English toggle, dark/light theme.
- **Single Binary** — Go binary with embedded UI, one-command Docker deploy.

---

## Requirements

- **Docker / Docker Compose** (recommended, no local environment needed)
- **Go**: `1.24+` (only for local builds, requires CGO)

---

## Quick Start

### Docker Compose (Recommended)

```yaml
services:
  context7-proxy:
    build: .
    ports:
      - "8070:8070"
    environment:
      - DATABASE_PATH=/app/data/proxy.db
      - CONTEXT7_BASE_URL=https://context7.com
      - UPSTREAM_TIMEOUT_SEC=30
      - COOLDOWN_SECONDS=60
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

```bash
git clone https://github.com/mydelren/context7-proxy.git
cd context7-proxy
docker compose up -d
```

### Docker CLI

```bash
docker build -t context7-proxy .
docker run -d \
  --name context7-proxy \
  -p 8070:8070 \
  -v $(pwd)/data:/app/data \
  context7-proxy
```

### Build from Source

```bash
CGO_ENABLED=1 go build -o context7-proxy .
./context7-proxy
```

---

## First Run

The Master Key is auto-generated on first start:

```bash
docker logs context7-proxy 2>&1 | grep "master key"
```

Log example: `level=INFO msg="no master key found, generated a new one" key=xxxxxxxx`

Open `http://<server-ip>:8070`, enter the Master Key, and add the team's `ctx7sk_...` keys in the Keys tab.

> Tip: Save the Master Key after first login. You can reset it from the Settings page.

---

## Connect Your Agents

Configure the proxy address in your agent's MCP client. All team members share the same endpoint:

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "env": {
        "CONTEXT7_API_URL": "http://<your-server-address>:8070"
      }
    }
  }
}
```

> **Note**: Replace `CONTEXT7_API_URL` with the actual address where your proxy is accessible.
>
> - Same machine: `http://127.0.0.1:8070`
> - LAN: `http://<lan-ip>:8070` (e.g., `http://192.168.1.100:8070`)
> - Remote: `https://<domain>` (e.g., `https://c7.example.com`)
>
> After logging into the management panel, the Settings page auto-generates the config based on your access URL — just copy and distribute to your team.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8070` | Listen address |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite database path |
| `CONTEXT7_BASE_URL` | `https://context7.com` | Upstream Context7 API URL |
| `UPSTREAM_TIMEOUT_SEC` | `30` | Upstream request timeout (seconds) |
| `COOLDOWN_SECONDS` | `60` | Cooldown after rate limit (seconds) |
| `MASTER_KEY` | auto-generated | Custom Master Key (optional) |

---

## License

MIT
