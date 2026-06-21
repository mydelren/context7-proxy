package services

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mydelren/context7-proxy/internal/models"
)

const context7MCPUpstream = "https://mcp.context7.com/mcp"

type MCPProxyService struct {
	auth      *AuthService
	keys      *KeyService
	logs      *LogService
	client    *http.Client
	upstream  string
}

func NewMCPProxyService(auth *AuthService, keys *KeyService, logs *LogService, upstreamTimeout time.Duration) *MCPProxyService {
	return &MCPProxyService{
		auth: auth,
		keys: keys,
		logs: logs,
		upstream: context7MCPUpstream,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 16,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				ResponseHeaderTimeout: upstreamTimeout,
			},
		},
	}
}

func (s *MCPProxyService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := extractMCPToken(r)
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	auth := s.auth.Validate(token)
	if auth.Role == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var assignedKeyID uint
	if auth.APIKeyID != nil {
		assignedKeyID = *auth.APIKeyID
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}

	reqID := uuid.NewString()[:8]
	ctx := r.Context()
	status, err := s.forward(ctx, reqID, r.Method, r.URL.Path, r.URL.RawQuery, r.Header, body, r.RemoteAddr, auth.MemberID, auth.MemberName, assignedKeyID, auth.Strategy, w)
	if err != nil {
		log.Printf("[%s] mcp forward error: %v", reqID, err)
		if status == 0 {
			writeJSONError(w, http.StatusInternalServerError, "mcp_proxy_error")
		}
		return
	}
}

func (s *MCPProxyService) forward(ctx context.Context, reqID, method, path, rawQuery string, headers http.Header, body []byte, clientIP string, memberID uint, memberName string, assignedKeyID uint, memberStrategy string, w http.ResponseWriter) (int, error) {
	if assignedKeyID > 0 {
		return s.doWithAssignedKey(ctx, reqID, method, path, rawQuery, headers, body, clientIP, memberID, memberName, assignedKeyID, w)
	}

	strategy := memberStrategy
	if strategy == "" {
		strategy = s.keys.GetStrategy(ctx)
	}
	candidates, err := s.keys.Candidates(ctx, strategy)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		writeJSONError(w, http.StatusServiceUnavailable, "no_available_keys")
		return http.StatusServiceUnavailable, nil
	}

	var last429 []byte
	for _, c := range candidates {
		status, respBody, latency, err := s.tryOnce(ctx, reqID, c.Key, method, path, rawQuery, headers, body)
		if err != nil {
			log.Printf("[%s] mcp key %d (%s) error: %v", reqID, c.ID, c.Alias, err)
			continue
		}
		switch status {
		case http.StatusUnauthorized:
			s.keys.MarkInvalid(ctx, c.ID)
			log.Printf("[%s] mcp key %d (%s) invalid (401)", reqID, c.ID, c.Alias)
			continue
		case http.StatusTooManyRequests:
			s.keys.MarkCooldown(ctx, c.ID)
			last429 = respBody
			log.Printf("[%s] mcp key %d (%s) rate limited (429)", reqID, c.ID, c.Alias)
			continue
		default:
			s.keys.MarkUsed(ctx, c.ID)
			s.logReq(ctx, reqID, c.ID, c.Alias, method, path, status, latency, clientIP, memberID, memberName)
			copyUpstreamResponse(w, status, nil, respBody)
			return status, nil
		}
	}

	if len(last429) > 0 {
		s.logReq(ctx, reqID, 0, "", method, path, http.StatusTooManyRequests, 0, clientIP, memberID, memberName)
		writeJSONBytes(w, http.StatusTooManyRequests, last429)
		return http.StatusTooManyRequests, nil
	}

	writeJSONError(w, http.StatusServiceUnavailable, "all_keys_failed")
	return http.StatusServiceUnavailable, nil
}

func (s *MCPProxyService) doWithAssignedKey(ctx context.Context, reqID, method, path, rawQuery string, headers http.Header, body []byte, clientIP string, memberID uint, memberName string, keyID uint, w http.ResponseWriter) (int, error) {
	k, err := s.keys.GetByID(ctx, keyID)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "assigned_key_not_found")
		return http.StatusServiceUnavailable, nil
	}
	if !k.IsActive || k.IsInvalid {
		writeJSONError(w, http.StatusServiceUnavailable, "assigned_key_unavailable")
		return http.StatusServiceUnavailable, nil
	}
	if k.CooldownAt != nil && k.CooldownAt.After(time.Now()) {
		writeJSONError(w, http.StatusTooManyRequests, "assigned_key_cooling_down")
		return http.StatusTooManyRequests, nil
	}
	if k.MaxRequests > 0 && k.UsedCount >= k.MaxRequests {
		writeJSONError(w, http.StatusTooManyRequests, "assigned_key_limit_reached")
		return http.StatusTooManyRequests, nil
	}

	status, respBody, latency, err := s.tryOnce(ctx, reqID, k.Key, method, path, rawQuery, headers, body)
	if err != nil {
		log.Printf("[%s] assigned mcp key %d (%s) error: %v", reqID, k.ID, k.Alias, err)
		writeJSONError(w, http.StatusBadGateway, "assigned_key_request_failed")
		return http.StatusBadGateway, nil
	}
	switch status {
	case http.StatusUnauthorized:
		s.keys.MarkInvalid(ctx, k.ID)
		writeJSONError(w, http.StatusServiceUnavailable, "assigned_key_invalid")
		return http.StatusServiceUnavailable, nil
	case http.StatusTooManyRequests:
		s.keys.MarkCooldown(ctx, k.ID)
		s.logReq(ctx, reqID, k.ID, k.Alias, method, path, status, latency, clientIP, memberID, memberName)
		writeJSONBytes(w, status, respBody)
		return status, nil
	default:
		s.keys.MarkUsed(ctx, k.ID)
		s.logReq(ctx, reqID, k.ID, k.Alias, method, path, status, latency, clientIP, memberID, memberName)
		copyUpstreamResponse(w, status, nil, respBody)
		return status, nil
	}
}

func (s *MCPProxyService) tryOnce(ctx context.Context, reqID, apiKey, method, path, rawQuery string, headers http.Header, body []byte) (int, []byte, int64, error) {
	upstreamPath := path
	if upstreamPath == "" || upstreamPath == "/" || upstreamPath == "/mcp" {
		upstreamPath = ""
	}
	u := s.upstream + upstreamPath
	if rawQuery != "" {
		u += "?" + rawQuery
	}

	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return 0, nil, 0, err
	}
	for k, vs := range headers {
		if shouldSkipForwardHeader(k) || strings.EqualFold(k, "CONTEXT7_API_KEY") {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("CONTEXT7_API_KEY", apiKey)

	start := time.Now()
	resp, err := s.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return 0, nil, latency, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusTooManyRequests:
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, latency, nil
	default:
		// Stream the upstream response body directly after the caller decides to accept it.
		b, err := io.ReadAll(resp.Body)
		return resp.StatusCode, b, latency, err
	}
}

func (s *MCPProxyService) logReq(ctx context.Context, reqID string, keyID uint, alias, method, endpoint string, status int, latency int64, clientIP string, memberID uint, memberName string) {
	if s.logs == nil {
		return
	}
	_ = s.logs.Create(ctx, &models.RequestLog{
		RequestID:  reqID,
		KeyID:      keyID,
		KeyAlias:   alias,
		MemberID:   memberID,
		MemberName: memberName,
		Method:     method,
		Endpoint:   endpoint,
		StatusCode: status,
		LatencyMs:  latency,
		ClientIP:   clientIP,
		CreatedAt:  time.Now(),
	})
}

func extractMCPToken(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("CONTEXT7_API_KEY")); token != "" {
		return token
	}
	if token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.URL.Query().Get("api_key")); token != "" {
		return token
	}
	return ""
}

func copyUpstreamResponse(w http.ResponseWriter, status int, headers http.Header, body []byte) {
	if headers != nil {
		for k, vs := range headers {
			if shouldSkipForwardHeader(k) {
				continue
			}
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
	}
	if w != nil {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if len(body) > 0 {
			_, _ = w.Write(body)
		}
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSONBytes(w, status, []byte(`{"error":"`+msg+`"}`))
}

func writeJSONBytes(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
