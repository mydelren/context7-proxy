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

This project solves all of that: the admin manages all API key lifecycles centrally, and team members access the proxy with individual Member Tokens — clear permissions, full audit trail.

---

## Features

- **Unified Endpoint** — all agents access Context7 API through one proxy address. No need to distribute real API keys.
- **Smart Key Pool** — automatically distributes requests across multiple keys for full quota utilization. Transparent failover on rate limits.
  - **Per-Key Limits** — set maximum requests per key (default 1000), auto-skip when exhausted.
  - **Manual Cooldown** — pause any key on demand, reusing the cooldown logic.
  - **Monthly Reset** — auto-reset all key usage counters on the 1st of each month.
- **Multi-Member Management**:
  - Admin account + Member Token system replaces single Master Key.
  - **Member Key Assignment** — assign specific API keys to members, or leave unassigned to follow global strategy.
  - **Per-Member Strategy** — each member can have their own key selection strategy (least used / round robin / random), falling back to global if unset.
  - Members can only use the proxy, not access the management panel.
  - Request logs track which member initiated each request.
- **Web Management UI**:
  - **Dashboard**: stat cards + 24h request chart for team-wide visibility.
  - **Keys**: add, delete, enable/disable API keys, set limits, manual cooldown, global strategy switch.
  - **Members**: create members, view tokens, assign keys, set strategy, delete members (admin-only).
  - **Logs**: detailed request logs with status code filter, shows member info.
  - **Settings**: view Master Key (legacy mode), change password, auto-generated MCP client config.
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

An admin account is auto-created on first start. The password is printed to the log:

```bash
docker logs context7-proxy 2>&1 | grep -i "admin account created"
```

Log example: `2026/06/01 10:01:48 Admin account created — username: admin, password: 5bb0a5175b31f7fa`

Open `http://<server-ip>:8070` and log in with `admin` and the password from the logs.

After login:
1. Change your admin password in **Settings**
2. Add your team's `ctx7sk_...` keys in the **Keys** tab
3. Create members in the **Members** tab — optionally assign specific keys and set per-member strategy, then get member tokens (for Agent configuration)
4. Distribute member tokens to team members for Agent setup

> Tip: Member tokens replace the old Master Key for proxy access. The Master Key still works for management panel login (legacy mode).

---

## Connect Your Agents

After creating members in the Members tab, configure the member token in your Agent's MCP client:

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "env": {
        "CONTEXT7_API_URL": "http://<your-server-address>:8070",
        "CONTEXT7_API_KEY": "<member-token>"
      }
    }
  }
}
```

> **Note**: Replace two values:
> - `CONTEXT7_API_URL`: the actual address where your proxy is accessible
>   - Same machine: `http://127.0.0.1:8070`
>   - LAN: `http://<lan-ip>:8070` (e.g., `http://192.168.1.100:8070`)
>   - Remote: `https://<domain>` (e.g., `https://c7.example.com`)
> - `CONTEXT7_API_KEY`: the member token created in the Members tab
>
> After logging into the management panel, the Settings page auto-generates the MCP config template.

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
