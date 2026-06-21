# Context7 Proxy MCP HTTP Exposure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `context7-proxy` as a direct MCP HTTP server on the same Go process, while keeping the existing Context7 API proxy, auth, logs, and admin UI intact.

**Architecture:** Keep the current Go + Gin + SQLite service as the single runtime. Add a direct `/mcp` endpoint to the existing Gin router so MCP clients can reach Context7 through the same process. The current implementation forwards MCP traffic to the official Context7 MCP upstream and keeps the existing proxy/key/log services, web UI, and admin API on the same port.

**Tech Stack:** Go 1.24, Gin, GORM, SQLite, Docker.

---

## File Structure

```
context7-proxy/
├── main.go
├── go.mod
├── go.sum
├── internal/
│   ├── config/config.go
│   ├── db/db.go
│   ├── httpserver/
│   │   ├── router.go
│   │   └── server.go
│   ├── mcp/
│   │   ├── server.go
│   │   ├── tools.go
│   │   └── auth.go
│   ├── models/models.go
│   └── services/
│       ├── auth_service.go
│       ├── key_service.go
│       ├── log_service.go
│       ├── master_key.go
│       ├── proxy_service.go
│       └── stats_service.go
├── README.md
├── README_EN.md
└── docs/
    └── active/system/CONTEXT7_PROXY_RUNBOOK.md
```

---

### Task 1: Lock the MCP contract and test surface

**Files:**
- Create: `internal/mcp/server_test.go`
- Create: `internal/mcp/tools_test.go`
- Modify: `README.md`
- Modify: `README_EN.md`

- [ ] **Step 1: Define the acceptance contract in tests**

Create tests that assert the MCP server exposes exactly two tools and that the server is mounted at `/mcp`:

```go
want := []string{"resolve-library-id", "query-docs"}
```

Use `httptest.NewServer` with the mounted handler and verify:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize", ...}
{"jsonrpc":"2.0","id":2,"method":"tools/list", ...}
```

The `tools/list` response must contain both tool names and no extra custom tools.

- [ ] **Step 2: Add one proxy-backed tool test**

Add a test that stubs the upstream call and verifies `resolve-library-id` returns a text result containing the selected library ID.

- [ ] **Step 3: Add one docs-backed tool test**

Add a test that stubs the upstream call and verifies `query-docs` returns the fetched documentation text for a supplied library ID.

- [ ] **Step 4: Record the expected runtime shape**

Update the docs to say the default runtime shape is now:

```text
http://127.0.0.1:8070/mcp
```

and note that this replaces the separate `@upstash/context7-mcp` wrapper in the normal path.

---

### Task 2: Add the MCP server package

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/auth.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the MCP SDK dependency**

Add `github.com/mark3labs/mcp-go` to `go.mod` so the service can speak MCP streamable HTTP directly.

- [ ] **Step 2: Mount `/mcp` in the existing HTTP process**

Mount the direct `/mcp` route inside the current Gin router with `http.Handler` or equivalent. Keep it in the same listener as the web UI and admin API.

- [ ] **Step 3: Preserve auth and logging**

Keep the member-token / legacy-master-key auth rules and request logging behavior unchanged for the `/mcp` path.

- [ ] **Step 4: Keep the implementation surface narrow**

Do not introduce a second runtime, wrapper daemon, or extra per-session MCP process in the normal path.

---

### Task 3: Wire MCP into the existing HTTP process

**Files:**
- Modify: `main.go`
- Modify: `internal/httpserver/router.go`
- Modify: `internal/httpserver/server.go`
- Modify: `internal/services/proxy_service.go`

- [ ] **Step 1: Instantiate the `/mcp` handler from `main.go`**

Create the `/mcp` handler alongside the current `AuthService`, `KeyService`, `LogService`, and `ProxyService`, and pass the same dependencies into it.

- [ ] **Step 2: Mount `/mcp` on the same listener**

Add one HTTP route on the existing Gin router for the MCP transport with `gin.WrapH(...)` or equivalent. Keep `/healthz`, `/manage/*`, and the current proxy routes unchanged.

- [ ] **Step 3: Keep auth behavior consistent**

Use the same member token / legacy master key rules for the MCP endpoint that the rest of the service already uses, so the new path does not create a second credential model.

- [ ] **Step 4: Relax the server timeout for streaming**

Change `http.Server` setup so the MCP path is not cut off by a fixed short `WriteTimeout`. Keep a read deadline for request headers if needed, but do not impose a 60s write timeout on the whole server if it breaks streamable HTTP.

---

### Task 4: Update runtime and docs for the new direct path

**Files:**
- Modify: `README.md`
- Modify: `README_EN.md`
- Create: `docs/mcp-http-runbook.md`

- [ ] **Step 1: Replace the old client snippet**

Change the documented Context7 client setup from the separate stdio wrapper to the direct HTTP MCP endpoint on this service.

- [ ] **Step 2: Document the operational boundary**

State clearly that the service still manages Context7 API keys and usage logs, but now also exposes MCP directly, so the wrapper process is no longer required in the normal path.

- [ ] **Step 3: Add rollback notes**

Document the rollback path: keep the existing API proxy and UI, and disable only the `/mcp` route if the new path needs to be taken out of service.

- [ ] **Step 4: Document failure classification**

Write three concrete failure buckets:

```text
transport failed -> MCP route/handler issue
unauthorized -> auth token / member token issue
upstream failed -> Context7 API / key pool issue
```

---

### Task 5: Verify real `/mcp` behavior end to end

**Files:**
- No new files; verification only

- [ ] **Step 1: Build the binary**

Run:

```bash
cd /home/lenovo/workspace/ops/context7-proxy
go build ./...
```

- [ ] **Step 2: Start the service and check the port**

Run the Docker compose stack or the local binary and verify `:8070` is listening.

- [ ] **Step 3: Verify the `/mcp` route is mounted**

Send a request against `/mcp` and confirm the route is served by the same listener as the UI and admin API.

- [ ] **Step 4: Confirm the existing proxy/UI still works**

Temporarily disable the `/mcp` route only, leave `/manage` and `/api|/v1` untouched, and confirm the existing proxy/UI still works.

---

## Self-Review

- The plan covers the current proxy, auth, logs, and UI and keeps them intact.
- The plan adds a direct MCP transport instead of a second wrapper process.
- The plan keeps rollback simple: remove or disable the MCP route, not the whole service.
