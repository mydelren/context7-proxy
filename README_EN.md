# Context7 API Key Proxy Pool

[简体中文](./README.md) | English

Pool multiple Context7 API keys behind a single proxy endpoint. Auto-rotates on rate limits, failover on errors, with a built-in web management UI.

---

## Features

- **Transparent Proxy** — forwards all Context7 API requests (`/v1/*`, `/api/*`) seamlessly.
- **Master Key Auth** — access via `Authorization: Bearer <MasterKey>`, no need to expose real API keys.
- **Smart Key Pool** — distributes requests by usage count (lowest first), randomizes ties to avoid burst limits.
- **Auto Failover** — switches to next available key on `429` (cooldown) and marks `401` as invalid.
- **Web Management UI**:
  - Dashboard: stat cards + 24h request chart.
  - Keys: add, delete, enable/disable.
  - Logs: detailed request logs with status filter and bulk clear.
  - Settings: view/reset Master Key, auto-generated MCP client config.
- **i18n & Themes** — Chinese/English toggle, dark/light theme.
- **Single Binary** — Go binary with embedded UI, one-command Docker deploy.

---

## Quick Start

### Docker Compose (Recommended)

Clone the repo and start:

```bash
git clone https://github.com/mydelren/context7-proxy.git
cd context7-proxy
docker compose up -d
```

The included `docker-compose.yml` builds the image automatically on first run.

To customize, edit the environment variables in `docker-compose.yml`:

### Docker CLI

Build the image first, then run:

```bash
docker build -t context7-proxy .
docker run -d \
  --name context7-proxy \
  -p 8070:8070 \
  -v $(pwd)/data:/app/data \
  context7-proxy
```

### Build from Source

Requires Go 1.24+ with CGO enabled (SQLite dependency):

```bash
CGO_ENABLED=1 go build -o context7-proxy .
./context7-proxy
```

> Note: If `go mod download` is slow, try setting `GOPROXY=https://goproxy.cn,direct` (or your regional proxy).

---

## First Run

The Master Key is auto-generated on first start. Retrieve it from logs:

```bash
docker logs context7-proxy 2>&1 | grep "master key"
```

Log example: `msg="no master key found, generated a new one" key=xxxxxxxx`

Open `http://<server-ip>:8070`, enter the Master Key, and add your `ctx7sk_...` keys in the Keys tab.

> Tip: Save the Master Key after first login. You can reset it from the Settings page.

---

## MCP Client Config

Connect this proxy to Claude Desktop, VS Code, or other MCP clients:

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

> **Note**: Replace `CONTEXT7_API_URL` with the actual address where your proxy is accessible. If the MCP client runs on the same machine, use `http://127.0.0.1:8070`. If deployed on a different device or remote server, use the corresponding IP or domain (e.g., `http://192.168.1.100:8070` or `https://c7.example.com`).
>
> After logging into the management panel, the Settings page auto-generates the config based on your current access URL — just copy and use it.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8070` | Listen address |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite database path |
| `CONTEXT7_BASE_URL` | `https://context7.com` | Upstream Context7 API URL |
| `UPSTREAM_TIMEOUT_SEC` | `30` | Upstream request timeout (seconds) |
| `COOLDOWN_SECONDS` | `60` | Cooldown after 429 (seconds) |
| `MASTER_KEY` | auto-generated | Custom Master Key (optional) |

---

## License

MIT
