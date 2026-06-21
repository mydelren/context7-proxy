package httpserver

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mydelren/context7-proxy/internal/services"
)

type dummyHandler struct{}

func (dummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func TestRouterMountsMCP(t *testing.T) {
	var fsys embed.FS
	router := NewRouter(Deps{
		Auth:     &services.AuthService{},
		Keys:     &services.KeyService{},
		Logs:     &services.LogService{},
		Stats:    &services.StatsService{},
		Proxy:    &services.ProxyService{},
		MCP:      &services.MCPProxyService{},
		StaticFS: fsys,
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatalf("expected /mcp to be mounted")
	}
}
