// internal/services/proxy_service.go
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

type ProxyService struct {
	baseURL string
	client  *http.Client
	keys    *KeyService
	logs    *LogService
}

func NewProxyService(baseURL string, timeout time.Duration, keys *KeyService, logs *LogService) *ProxyService {
	return &ProxyService{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
		keys:    keys,
		logs:    logs,
	}
}

type ProxyResponse struct {
	StatusCode int
	Body       []byte
}

func (p *ProxyService) Do(ctx context.Context, method, path, rawQuery string, headers http.Header, body []byte, clientIP string, memberID uint, memberName string) (ProxyResponse, error) {
	reqID := uuid.NewString()[:8]
	candidates, err := p.keys.Candidates(ctx)
	if err != nil {
		return ProxyResponse{}, err
	}
	if len(candidates) == 0 {
		return ProxyResponse{StatusCode: 503, Body: []byte(`{"error":"no_available_keys"}`)}, nil
	}

	var lastResp ProxyResponse
	for _, c := range candidates {
		respBody, status, latency, err := p.tryKey(ctx, c.Key, method, path, rawQuery, headers, body)
		if err != nil {
			log.Printf("[%s] key %d (%s) error: %v", reqID, c.ID, c.Alias, err)
			continue
		}
		switch status {
		case 401:
			log.Printf("[%s] key %d (%s) invalid (401)", reqID, c.ID, c.Alias)
			p.keys.MarkInvalid(ctx, c.ID)
			continue
		case 429:
			log.Printf("[%s] key %d (%s) rate limited (429)", reqID, c.ID, c.Alias)
			p.keys.MarkCooldown(ctx, c.ID)
			lastResp = ProxyResponse{StatusCode: 429, Body: respBody}
			continue
		}
		p.keys.MarkUsed(ctx, c.ID)
		p.logReq(ctx, reqID, c.ID, c.Alias, method, path, status, latency, clientIP, memberID, memberName)
		return ProxyResponse{StatusCode: status, Body: respBody}, nil
	}

	if lastResp.StatusCode == 429 {
		p.logReq(ctx, reqID, 0, "", method, path, 429, 0, clientIP, memberID, memberName)
		return lastResp, nil
	}
	return ProxyResponse{StatusCode: 503, Body: []byte(`{"error":"all_keys_failed"}`)}, nil
}

func (p *ProxyService) tryKey(ctx context.Context, apiKey, method, path, rawQuery string, headers http.Header, body []byte) ([]byte, int, int64, error) {
	u := p.baseURL + path
	if rawQuery != "" {
		u += "?" + rawQuery
	}
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, 0, 0, err
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, 0, latency, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, latency, err
}

func (p *ProxyService) logReq(ctx context.Context, reqID string, keyID uint, alias, method, endpoint string, status int, latency int64, clientIP string, memberID uint, memberName string) {
	if p.logs == nil {
		return
	}
	p.logs.Create(ctx, &models.RequestLog{
		RequestID: reqID, KeyID: keyID, KeyAlias: alias,
		MemberID: memberID, MemberName: memberName,
		Method: method, Endpoint: endpoint, StatusCode: status, LatencyMs: latency,
		ClientIP: clientIP, CreatedAt: time.Now(),
	})
}
