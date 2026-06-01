# Context7 API Key 代理池

简体中文 | [English](./README_EN.md)

将多个 Context7 API Key 汇聚在一个 **Master Key** 之后，自动轮换、故障切换，并提供内置 Web 管理面板。

---

## 功能特性

- **透明代理**：完整转发至 `https://context7.com`（支持所有路径与方法）。
- **Master Key 鉴权**：客户端通过 `Authorization: Bearer <MasterKey>` 安全访问，无需暴露真实 API Key。
- **智能 Key 池管理**：
  - 按使用次数优先分配，均衡负载。
  - 同次数 Key 随机打散，有效防止请求过于集中触发频率限制。
- **自动故障切换**：遇到 `429` 自动冷却并切换下一个 Key；`401` 标记为无效。
- **MCP 支持**：内置 HTTP MCP（Model Context Protocol）端点，可接入 Claude Desktop、VS Code 等 AI 工具。
- **Web 管理面板**：
  - **仪表盘**：统计卡片 + 24h 请求量图表。
  - **密钥管理**：添加、删除、启用/禁用 API Key。
  - **请求日志**：详细记录每次请求，支持状态码过滤与清空。
  - **设置**：查看/重置 Master Key，MCP 客户端配置（自动根据访问地址生成）。
- **中英文 / 深浅色**：支持中英文切换和深色/浅色主题。
- **开箱即用**：Go 单二进制，内嵌 Web UI，Docker 一键部署。

---

## 环境要求

- **Docker / Docker Compose**（推荐，无需本地环境）
- **Go**: `1.24+`（仅本地编译需要，依赖 CGO）

---

## 快速部署

### Docker Compose（推荐）

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

克隆仓库后直接启动：

```bash
git clone https://github.com/mydelren/context7-proxy.git
cd context7-proxy
docker compose up -d
```

### Docker 原生命令

```bash
docker build -t context7-proxy .
docker run -d \
  --name context7-proxy \
  -p 8070:8070 \
  -v $(pwd)/data:/app/data \
  context7-proxy
```

### 本地编译

需要 Go 1.24+，SQLite 依赖 CGO：

```bash
CGO_ENABLED=1 go build -o context7-proxy .
./context7-proxy
```

---

## 首次运行

服务在**首次启动**时会自动生成一个随机的 **Master Key**，用于登录管理面板和调用 API。

查看日志获取 Master Key：

```bash
docker logs context7-proxy 2>&1 | grep "master key"
```

日志示例：`level=INFO msg="no master key found, generated a new one" key=xxxxxxxx`

打开 `http://<服务器IP>:8070`，输入 Master Key 登录，在「密钥管理」中添加你的 `ctx7sk_...` Key。

> 提示：建议首次登录后在「设置」页面妥善保存 Master Key。可在设置页面重置。

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

> **注意**：`CONTEXT7_API_URL` 需要替换为你的代理服务实际可访问地址。
>
> - 同一台机器：`http://127.0.0.1:8070`
> - 局域网其他设备：`http://<局域网IP>:8070`（如 `http://192.168.1.100:8070`）
> - 远程服务器：`https://<域名>`（如 `https://c7.example.com`）
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
