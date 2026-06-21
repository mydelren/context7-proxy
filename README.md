# Context7 API 代理管理器

简体中文 | [English](./README_EN.md)

为团队和多 Agent 场景提供统一的 Context7 API 入口，集中管理密钥、监控用量、控制访问权限，并直接暴露 `/mcp`。

---

## 为什么需要这个？

当团队多人使用 AI Agent（Claude Code、Cursor、VS Code 等）接入 Context7 文档服务时，每人各自配置 API Key 会带来：

- **密钥分散**：每个人各自申请、各自保存，难以统一管理
- **用量不可见**：谁在用、用了多少、是否接近限额，没有全局视图
- **切换成本高**：轮换 Key 需要通知所有人更新配置
- **访问控制缺失**：无法撤销某人的访问权限而不影响其他人

本项目解决以上问题：管理员统一管理所有 API Key，团队成员通过独立的 Member Token 接入代理，权限清晰、审计可追溯。

---

## 功能特性

- **统一入口**：所有 Agent 通过同一个代理地址访问 Context7 API，无需各自配置真实 Key。
- **直接 MCP 暴露**：本服务同时提供 `/mcp`，同进程接入 MCP 客户端并转发到官方 Context7 MCP 上游。
- **智能密钥池**：
  - 自动均衡分配请求到多个 Key，充分利用额度。
  - 遇到限流自动冷却切换，对调用方完全透明。
  - **Per-Key 限额**：为每个 Key 设置最大请求次数（默认 1000），到限自动跳过。
  - **手动冷却**：一键暂停指定 Key，复用冷却逻辑。
  - **月度重置**：每月 1 日自动重置所有 Key 的使用计数。
- **多成员管理**：
  - 管理员账号 + 成员 Token 体系，替代单一 Master Key。
  - **成员指定密钥**：可为成员绑定专属 API Key，未指定则走全局策略。
  - **成员独立策略**：每个成员可设置自己的 key 选择策略（最少使用/轮询/随机），未设置则跟随全局。
  - 成员只能使用代理，不能访问管理面板。
  - 请求日志记录哪个成员发起的请求。
- **Web 管理面板**：
  - **仪表盘**：统计卡片 + 24h 请求量图表，团队用量一目了然。
  - **密钥管理**：添加、删除、启用/禁用 API Key，设置限额，手动冷却，全局策略切换。
  - **成员管理**：创建成员、查看 Token、指定密钥、设置策略、删除成员（管理员可见）。
  - **请求日志**：详细记录每次请求，支持状态码过滤，显示成员信息。
  - **设置**：查看 Master Key（兼容模式），修改密码，自动生成 MCP 客户端配置。
- **中英文 / 深浅色**：支持中英文切换和深色/浅色主题。
- **开箱即用**：Go 单二进制，内嵌 Web UI，Docker 一键部署。

---

## 环境要求

- **Docker / Docker Compose**（推荐，无需本地环境）

---

## 快速部署

### Docker Compose（推荐）

```yaml
services:
  context7-proxy:
    image: ghcr.io/mydelren/context7-proxy:latest
    ports:
      - "8070:8070"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai
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
  -e TZ=Asia/Shanghai \
  ghcr.io/mydelren/context7-proxy:latest
```

### 本地编译（可选）

```bash
git clone https://github.com/mydelren/context7-proxy.git
cd context7-proxy
CGO_ENABLED=1 go build -o context7-proxy .
./context7-proxy
```

---

## 首次运行

服务首次启动时自动创建管理员账号，密码打印到日志：

```bash
docker logs context7-proxy 2>&1 | grep -i "admin account created"
```

日志示例：`2026/06/01 10:01:48 Admin account created — username: admin, password: 5bb0a5175b31f7fa`

打开 `http://<服务器IP>:8070`，使用 `admin` 和日志中的密码登录。

登录后：
1. 在「设置」中修改管理员密码
2. 在「密钥管理」中添加团队的 `ctx7sk_...` Key
3. 在「成员管理」中创建成员，可指定专属密钥和选择策略，获得成员 Token（用于 Agent 配置）
4. 将成员 Token 分发给团队成员配置 Agent

> 提示：成员 Token 替代了旧的 Master Key 作为代理访问凭证。Master Key 仍可用于登录管理面板（兼容模式）。

---

## 接入 Agent

管理员在「成员管理」中创建成员后，将获得的 Token 配置到 Agent 的 MCP 客户端中：

```json
{
  "mcpServers": {
    "context7": {
      "url": "http://<你的服务器地址>:8070/mcp",
      "headers": {
        "CONTEXT7_API_KEY": "<成员Token>"
      }
    }
  }
}
```

> **注意**：需要替换两个值：
> - `url`：代理服务的实际可访问地址
>   - 同一台机器：`http://127.0.0.1:8070/mcp`
>   - 局域网内：`http://<局域网IP>:8070/mcp`（如 `http://192.168.1.100:8070/mcp`）
>   - 远程服务器：`https://<域名>/mcp`（如 `https://c7.example.com/mcp`）
> - `CONTEXT7_API_KEY`：管理员在「成员管理」中创建的成员 Token
>
> 登录管理面板后，「设置」页面会自动根据当前访问地址生成 MCP 配置模板。

---

## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `LISTEN_ADDR` | `:8070` | 监听地址 |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite 数据库路径 |
| `CONTEXT7_BASE_URL` | `https://context7.com` | 上游 Context7 API 地址 |
| `UPSTREAM_TIMEOUT_SEC` | `30` | 上游请求超时（秒） |
| `COOLDOWN_SECONDS` | `60` | 限流后冷却时间（秒） |
| `MASTER_KEY` | 自动生成 | 自定义 Master Key（可选） |

---

## 许可证

MIT
