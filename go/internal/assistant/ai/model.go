package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lab.trilha/go/internal/assistant/metrics"
	"net"
	"net/http"
	"strings"
	"time"
)

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

func (m Message) Text() *string {
	if value, ok := m.Content.(string); ok {
		return &value
	}
	return nil
}

type ProviderError struct {
	Status int
	Kind   string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("falha no provedor: %s, HTTP %d", e.Kind, e.Status)
}

type Completion interface {
	Chat(context.Context, string, []Message, []any) (Message, error)
}
type OpenAIClient struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	Metrics    *metrics.Counters
	Parameters map[string]any
}

func NewClient(base, key string, timeout time.Duration, counters *metrics.Counters) *OpenAIClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Keep connection settings aligned with the Java agent's JDK 21 HTTP client.
	transport.Proxy = nil
	transport.IdleConnTimeout = 30 * time.Second
	transport.MaxIdleConns = 0
	transport.MaxIdleConnsPerHost = int(^uint(0) >> 1)
	transport.MaxConnsPerHost = 0
	transport.DialContext = (&net.Dialer{Timeout: 60 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 0
	httpClient := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &OpenAIClient{BaseURL: strings.TrimRight(base, "/"), APIKey: key, HTTP: httpClient, Metrics: counters, Parameters: map[string]any{"temperature": 0.0, "max_tokens": 512, "stream": false}}
}

func (c *OpenAIClient) Chat(ctx context.Context, model string, messages []Message, tools []any) (result Message, err error) {
	c.Metrics.Requests.Add(1)
	defer func() {
		if err != nil {
			c.Metrics.Errors.Add(1)
		}
	}()
	body := map[string]any{"model": model, "messages": messages}
	for key, value := range c.Parameters {
		body[key] = value
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return Message{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return Message{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	response, err := c.HTTP.Do(request)
	if err != nil {
		return Message{}, &ProviderError{Kind: "transport"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		io.Copy(io.Discard, response.Body)
		return Message{}, &ProviderError{Status: response.StatusCode, Kind: "http"}
	}
	var completion struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage *struct {
			Input  *int64 `json:"prompt_tokens"`
			Output *int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	responseBytes, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return Message{}, &ProviderError{Kind: "transport"}
	}
	if err = json.Unmarshal(responseBytes, &completion); err != nil || len(completion.Choices) == 0 {
		return Message{}, &ProviderError{Kind: "invalid-response"}
	}
	result = completion.Choices[0].Message
	c.Metrics.Responses.Add(1)
	if completion.Usage == nil || completion.Usage.Input == nil || completion.Usage.Output == nil {
		c.Metrics.MissingUsage.Add(1)
	}
	if completion.Usage != nil {
		if completion.Usage.Input != nil {
			c.Metrics.InputTokens.Add(*completion.Usage.Input)
		}
		if completion.Usage.Output != nil {
			c.Metrics.OutputTokens.Add(*completion.Usage.Output)
		}
	}
	c.Metrics.ToolCalls.Add(int64(len(result.ToolCalls)))
	return result, nil
}
