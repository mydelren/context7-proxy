package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mydelren/context7-proxy/internal/db"
)

func TestMCPProxyService_ForwardsSessionHeadersAndNormalizesPath(t *testing.T) {
	const (
		memberToken  = "member-token-123"
		upstreamKey  = "upstream-key-abc"
		sessionID    = "session-xyz"
	)

	gormDB, err := db.Open(t.TempDir() + "/proxy.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	auth := NewAuthService(gormDB, "")
	keys := NewKeyService(gormDB, 60)
	logs := NewLogService(gormDB)

	member, err := auth.CreateMember(context.Background(), "MO5")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	member.Token = memberToken
	if err := gormDB.Save(member).Error; err != nil {
		t.Fatalf("save member token: %v", err)
	}

	key, err := keys.Create(context.Background(), upstreamKey, "primary")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	_ = key

	reqCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		if r.URL.Path != "/mcp" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if got := r.Header.Get("CONTEXT7_API_KEY"); got != upstreamKey {
			t.Fatalf("unexpected upstream api key: %q", got)
		}

		switch reqCount {
		case 1:
			if got := r.Header.Get("Mcp-Session-Id"); got != "" {
				t.Fatalf("initialize request should not send session id, got %q", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Mcp-Session-Id", sessionID)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2025-03-26\"}}\n\n"))
		case 2:
			if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
				t.Fatalf("tools/list request missing session id, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", sessionID)
			w.WriteHeader(http.StatusOK)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      2,
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "resolve-library-id"},
						{"name": "query-docs"},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		default:
			t.Fatalf("unexpected upstream request count: %d", reqCount)
		}
	}))
	defer upstream.Close()

	svc := NewMCPProxyService(auth, keys, logs, 5*time.Second)
	svc.upstream = upstream.URL + "/mcp"

	initBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"codex","version":"test"}}}`)
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBody))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initReq.Header.Set("MCP-Protocol-Version", "2025-03-26")
	initReq.Header.Set("CONTEXT7_API_KEY", memberToken)
	initRR := httptest.NewRecorder()

	svc.ServeHTTP(initRR, initReq)

	if initRR.Code != http.StatusOK {
		t.Fatalf("unexpected initialize status: %d, body=%s", initRR.Code, initRR.Body.String())
	}
	if got := initRR.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("initialize content-type not forwarded: %q", got)
	}
	if got := initRR.Header().Get("Mcp-Session-Id"); got != sessionID {
		t.Fatalf("initialize session id not forwarded: %q", got)
	}

	listBody := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	listReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(listBody))
	listReq.Header.Set("Content-Type", "application/json")
	listReq.Header.Set("Accept", "application/json, text/event-stream")
	listReq.Header.Set("MCP-Protocol-Version", "2025-03-26")
	listReq.Header.Set("CONTEXT7_API_KEY", memberToken)
	listReq.Header.Set("Mcp-Session-Id", sessionID)
	listRR := httptest.NewRecorder()

	svc.ServeHTTP(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("unexpected tools/list status: %d, body=%s", listRR.Code, listRR.Body.String())
	}
	if got := listRR.Header().Get("Mcp-Session-Id"); got != sessionID {
		t.Fatalf("tools/list session id not forwarded: %q", got)
	}
	if got := listRR.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("tools/list content-type not forwarded: %q", got)
	}
}

func TestMCPProxyService_ForwardsHeadersOnRateLimit(t *testing.T) {
	const (
		memberToken = "member-token-429"
		upstreamKey = "upstream-key-429"
		sessionID   = "rate-limit-session"
	)

	gormDB, err := db.Open(t.TempDir() + "/proxy.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	auth := NewAuthService(gormDB, "")
	keys := NewKeyService(gormDB, 60)
	logs := NewLogService(gormDB)

	member, err := auth.CreateMember(context.Background(), "MO5")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	member.Token = memberToken
	if err := gormDB.Save(member).Error; err != nil {
		t.Fatalf("save member token: %v", err)
	}

	if _, err := keys.Create(context.Background(), upstreamKey, "primary"); err != nil {
		t.Fatalf("create key: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", sessionID)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("data: {\"error\":\"rate_limited\"}\n\n"))
	}))
	defer upstream.Close()

	svc := NewMCPProxyService(auth, keys, logs, 5*time.Second)
	svc.upstream = upstream.URL + "/mcp"

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"codex","version":"test"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-03-26")
	req.Header.Set("CONTEXT7_API_KEY", memberToken)
	rr := httptest.NewRecorder()

	svc.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d, body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("rate-limit content-type not forwarded: %q", got)
	}
	if got := rr.Header().Get("Mcp-Session-Id"); got != sessionID {
		t.Fatalf("rate-limit session id not forwarded: %q", got)
	}
}
