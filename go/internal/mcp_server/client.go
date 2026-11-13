package mcp_server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client manages MCP sessions and tool calls over Streamable HTTP.
type Client struct {
	URL       string
	HTTP      *http.Client
	Session   string
	next      atomic.Int64
	Tools     []any
	sessionMu sync.RWMutex
	stop      context.CancelFunc
}

func NewClient(ctx context.Context, url string, timeout time.Duration) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Keep connection settings aligned with the Java agent's JDK 21 HTTP client.
	transport.Proxy = nil
	transport.IdleConnTimeout = 30 * time.Second
	transport.MaxIdleConns = 0
	transport.MaxIdleConnsPerHost = int(^uint(0) >> 1)
	transport.MaxConnsPerHost = 0
	transport.DialContext = (&net.Dialer{Timeout: 60 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 0
	c := &Client{URL: url, HTTP: &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	_, err := c.call(ctx, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "trilha-go", "version": "1.0.0"}}, false)
	if err != nil {
		return nil, err
	}
	if _, err = c.call(ctx, "notifications/initialized", map[string]any{}, true); err != nil {
		return nil, err
	}
	listing, err := c.call(ctx, "tools/list", map[string]any{}, false)
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err = json.Unmarshal(listing, &result); err != nil {
		return nil, err
	}
	for _, tool := range result.Tools {
		c.Tools = append(c.Tools, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.InputSchema}})
	}
	c.startHealthCheck()
	return c, nil
}
func (c *Client) Close() {
	if c.stop != nil {
		c.stop()
	}
	c.HTTP.CloseIdleConnections()
}
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	raw, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, false)
	if err != nil {
		return "", err
	}
	var result struct {
		IsError    bool            `json:"isError"`
		Structured json.RawMessage `json:"structuredContent"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if result.IsError {
		return "", errors.New("tool MCP retornou isError")
	}
	if len(result.Structured) > 0 && string(result.Structured) != "null" {
		var compact bytes.Buffer
		if err = json.Compact(&compact, result.Structured); err != nil {
			return "", err
		}
		return compact.String(), nil
	}
	parts := []string{}
	for _, block := range result.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}
func (c *Client) call(ctx context.Context, method string, params any, notification bool) (json.RawMessage, error) {
	body := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if !notification {
		body["id"] = c.next.Add(1)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if method != "initialize" {
		request.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	c.sessionMu.RLock()
	sid := c.Session
	c.sessionMu.RUnlock()
	if sid != "" {
		request.Header.Set("MCP-Session-Id", sid)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, errors.New("falha de transporte MCP")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP HTTP %d", response.StatusCode)
	}
	if sid := response.Header.Get("MCP-Session-Id"); sid != "" && method == "initialize" {
		c.sessionMu.Lock()
		c.Session = sid
		c.sessionMu.Unlock()
	}
	if notification {
		return nil, nil
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		data := []string{}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		raw = []byte(strings.Join(data, "\n"))
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("resposta MCP inválida")
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return nil, errors.New("erro JSON-RPC MCP")
	}
	return envelope.Result, nil
}

// Use DefaultMcpClient's health-check cadence: first ping after 30s, then every 30s.
// Reinitialize the session on failure, retrying every 5s.
func (c *Client) startHealthCheck() {
	ctx, cancel := context.WithCancel(context.Background())
	c.stop = cancel
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := c.call(ctx, "ping", map[string]any{}, false); err == nil {
					continue
				}
				for {
					c.sessionMu.Lock()
					c.Session = ""
					c.sessionMu.Unlock()
					_, err := c.call(ctx, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "trilha-go", "version": "1.0.0"}}, false)
					if err == nil {
						_, err = c.call(ctx, "notifications/initialized", map[string]any{}, true)
						if err == nil {
							break
						}
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
					}
				}
			}
		}
	}()
}
