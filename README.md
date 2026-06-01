# Context7 API Key Proxy

Pool multiple Context7 API keys behind a single endpoint. Auto-rotates on 429 rate limits. Web management UI included.

## Quick Start

```bash
docker compose up -d
docker compose logs | grep "master key"
```

Open `http://localhost:8070`, enter the master key, add your `ctx7sk_...` keys.

## MCP Client Config

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "env": {
        "CONTEXT7_API_URL": "http://127.0.0.1:8070"
      }
    }
  }
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8070` | Listen address |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite path |
| `CONTEXT7_BASE_URL` | `https://context7.com` | Upstream URL |
| `UPSTREAM_TIMEOUT_SEC` | `30` | Timeout (sec) |
| `COOLDOWN_SECONDS` | `60` | 429 cooldown |
| `MASTER_KEY` | auto | Custom master key |

## License

MIT
