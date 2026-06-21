# Context7 Proxy MCP HTTP 修复与交付计划

> 这份计划只针对当前已经存在的 `context7-proxy` 单进程 `/mcp` 直出链路做收敛修复与交付，不重做一套本地 MCP server，不新增第二运行时。

## 目标

在保留现有 `context7-proxy` 单进程架构、数据库、管理 UI、成员 token、密钥池和 API 代理能力的前提下，完成以下交付：

1. 修复 `/mcp` 上游路径重复拼接问题，避免请求被转发到 `.../mcp/mcp`
2. 修复 MCP 上游响应头透传问题，至少保证 `Content-Type` 与 `Mcp-Session-Id` 不丢
3. 保持本地运行形态为：
   - 管理/UI：`http://127.0.0.1:8070`
   - MCP：`http://127.0.0.1:19187/mcp`
   - 实现方式：同一容器进程监听 `8070`，host 额外映射 `19187 -> 8070`
4. 保留现有持久化数据和 live 配置，不重置数据库、不重建 token
5. 最终让 Codex 和 Claude 都能通过 `http://127.0.0.1:19187/mcp` 做真实 MCP 调用

## 范围边界

本次允许修改：

- `internal/services/mcp_proxy_service.go`
- 针对上述修复的最小回归测试
- 仓库内 README / compose 示例 / 运行文档
- live compose 的 host 端口映射与镜像更新

本次明确不做：

- 不引入 `mcp-go`
- 不新增 `internal/mcp/*`
- 不把当前实现改造成“本地自定义工具集合”
- 不重写成员认证模型
- 不迁移数据库或变更数据目录

## 已知 P0 问题

### P0-1 上游路径重复

当前 `/mcp` 请求在某些情况下会被转发成：

```text
https://mcp.context7.com/mcp/mcp
```

正确行为应为：

```text
https://mcp.context7.com/mcp
```

### P0-2 上游响应头丢失

当前代理曾出现只转发 body、不转发上游响应头的问题，导致 MCP 客户端丢失：

- `Content-Type`
- `Mcp-Session-Id`

这会直接破坏 streamable HTTP MCP 会话连续性。

## 执行步骤

### 1. 代码修复

- 在 `MCPProxyService.tryOnce(...)` 中：
  - 归一化 `""`、`"/"`、`"/mcp"`，避免上游重复拼接
  - 返回上游 `http.Header`
- 在 `forward(...)` 与 `doWithAssignedKey(...)` 中：
  - 成功响应透传上游 header + body
  - `429` 响应也透传上游 header + body
- 保持 `/manage/*`、静态 UI、现有 `/api|/v1` 代理不变

### 2. 回归测试

至少覆盖：

1. `/mcp` 不会被转发成 `/mcp/mcp`
2. `initialize` 成功响应时会透传：
   - `Content-Type`
   - `Mcp-Session-Id`
3. `429` 响应时也会透传：
   - `Content-Type`
   - `Mcp-Session-Id`

### 3. 发布前门禁

发布前必须记录：

1. 当前 live compose 备份
2. 当前 live 镜像 digest
3. `GET /healthz` 正常
4. live 数据库仍存在成员 token，且不做重置

### 4. 镜像发布

1. 提交代码
2. `git push origin master`（必要时走本机 `7890` 代理）
3. 等待 GitHub Actions 的 Go CI 与镜像构建完成
4. 确认目标 commit 对应镜像已生成，再动 live

### 5. Live 更新

更新 live compose：

- 保留 `8070:8070`
- 增加 `19187:8070`
- 保持 `./data:/app/data`
- 使用新镜像 `ghcr.io/mydelren/context7-proxy:latest`

然后：

```bash
docker compose pull
docker compose up -d
```

### 6. Live 验证

必须做真实验证，不能只看配置：

1. `http://127.0.0.1:8070/healthz` 返回 200
2. `http://127.0.0.1:8070` 管理/UI 仍可访问
3. 对 `http://127.0.0.1:19187/mcp` 用真实 member token 发 MCP initialize
   - 断言 `Content-Type`
   - 断言 `Mcp-Session-Id`
4. 用同一 session 做 `tools/list`
5. 再做一次真实 `tools/call`
   - `resolve-library-id`
   - 或 `query-docs`
6. 再用真实客户端验证：
   - Codex Context7 MCP
   - Claude Context7 MCP

## 回滚

如 live 更新后异常，按以下顺序回滚：

1. 使用已备份的 compose 恢复
2. 回到更新前镜像 digest
3. 保留 `./data/proxy.db` 原样，不删库
4. 保证最少恢复到：
   - `8070` 管理/UI 正常
   - 数据和 token 仍在

## 通过标准

只有同时满足以下条件，才算本次交付完成：

1. 仓库测试通过
2. GitHub Actions 构建通过
3. live 容器已更新到新镜像
4. `8070` UI 正常
5. `19187/mcp` 真实 initialize / tools/list / tools/call 成功
6. Codex 与 Claude 的 Context7 MCP 都能实际调用
