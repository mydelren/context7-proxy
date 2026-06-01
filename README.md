# Context7 API Key 代理池

简体中文 | [English](./README_EN.md)

将多个 Context7 API Key 汇聚在一个代理端点之后，自动轮换、故障切换，并提供内置 Web 管理面板。

---

## 功能特性

- **透明代理**：完整转发 Context7 API 请求（`/v1/*`、`/api/*`），客户端无感知。
- **Master Key 鉴权**：通过 `Authorization: Bearer <MasterKey>` 安全访问，无需暴露真实 API Key。
- **智能 Key 池管理**：
  - 按使用次数优先分配，均衡负载。
  - 同次数 Key 随机打散，防止请求集中触发限流。
- **自动故障切换**：遇到 `429` 自动冷却并切换下一个 Key；`401` 标记为无效。
- **Web 管理面板**：
  - 仪表盘：统计卡片 + 24h 请求量图表。
  - 密钥管理：添加、删除、启用/禁用。
  - 请求日志：详细记录每次请求，支持状态码过滤与清空。
  - 设置：查看/重置 Master Key，MCP 客户端配置。
- **中英文 / 深浅色**：支持中英文切换和深色/浅色主题。
- **开箱即用**：Go 单二进制，内嵌 Web UI，Docker 一键部署。

---

## 快速部署

### Docker Compose（推荐）

```yaml
services:
  context7-proxy:
    image: ghcr.io/mydelren/context7-proxy:latest
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
docker compose up -d
```

### Docker 原生命令

```bash
docker run -d \
  --name context7-proxy \
  -p 8070:8070 \
  -v $(pwd)/data:/app/data \
  ghcr.io/mydelren/context7-proxy:latest
```

### 本地编译

```bash
go build -o context7-proxy .
./context7-proxy
```

---

## 首次运行

服务首次启动时自动生成 Master Key，查看日志获取：

```bash
docker logs context7-proxy 2>&1 | grep "master key"
```

日志示例：`msg="no master key found, generated a new one" key=xxxxxxxx`

打开 `http://<服务器IP>:8070`，输入 Master Key 登录，在「密钥管理」中添加你的 `ctx7sk_...` Key。

> 提示：首次登录后建议妥善保存 Master Key，可在「设置」页面重置。

---

## MCP 客户端配置

将本代理接入 Claude Desktop、VS Code 等 MCP 客户端：

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "env": {
        "CONTEXT7_API_URL": "http://<你的服务器地址>:8070"
      }
    }
  }
}
```

> **注意**：`CONTEXT7_API_URL` 需要替换为你的代理服务实际可访问地址。如果 MCP 客户端与代理运行在同一台机器上，可使用 `http://127.0.0.1:8070`；如果部署在不同设备或远程服务器，请填写对应 IP 或域名（如 `http://192.168.1.100:8070` 或 `https://c7.example.com`）。
>
> 登录管理面板后，「设置」页面会自动根据当前访问地址生成配置，可直接复制使用。

---

## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `LISTEN_ADDR` | `:8070` | 监听地址 |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite 数据库路径 |
| `CONTEXT7_BASE_URL` | `https://context7.com` | 上游 Context7 API 地址 |
| `UPSTREAM_TIMEOUT_SEC` | `30` | 上游请求超时（秒） |
| `COOLDOWN_SECONDS` | `60` | 429 后冷却时间（秒） |
| `MASTER_KEY` | 自动生成 | 自定义 Master Key（可选） |

---

## 许可证

MIT
