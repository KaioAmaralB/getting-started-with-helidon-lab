package mcp_server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"lab.trilha/go/internal/assistant/config"
	"lab.trilha/go/internal/domain"
	"net/http"
	"sync"
)

const resultSchema = `{"type":"object","oneOf":[{"type":"object","title":"Estado de um pedido fictício da Trilha","properties":{"orderId":{"type":"string"},"item":{"type":"string"},"status":{"type":"string"},"deliveryEstimate":{"type":"string"}},"required":["orderId","item","status","deliveryEstimate"],"additionalProperties":false},{"type":"object","title":"Consulta negada pela política da Trilha","properties":{"error":{"type":"string","enum":["INVALID_ORDER_ID","ORDER_NOT_ALLOWED"]}},"required":["error"],"additionalProperties":false}]}`

type Server struct {
	tool     map[string]any
	mu       sync.Mutex
	sessions map[string]bool
}

func NewServer(contract config.Contract) (*Server, error) {
	var definition struct {
		Function struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(contract.Tools["mcp"], &definition); err != nil {
		return nil, err
	}
	tool := map[string]any{"name": definition.Function.Name, "description": definition.Function.Description, "inputSchema": definition.Function.Parameters, "outputSchema": json.RawMessage(resultSchema), "annotations": map[string]any{"title": "", "readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}}
	return &Server{tool: tool, sessions: map[string]bool{}}, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orders" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		s.mu.Lock()
		delete(s.sessions, r.Header.Get("MCP-Session-Id"))
		s.mu.Unlock()
		w.WriteHeader(200)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(405)
		return
	}
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		rpcError(w, nil, -32700, "Parse error", 400)
		return
	}
	if request.JSONRPC != "2.0" || request.Method == "" {
		rpcError(w, request.ID, -32600, "Invalid Request", 400)
		return
	}
	if request.Method != "initialize" {
		sid := r.Header.Get("MCP-Session-Id")
		if sid == "" {
			rpcError(w, request.ID, -32600, "Missing session", 400)
			return
		}
		s.mu.Lock()
		known := s.sessions[sid]
		s.mu.Unlock()
		if !known {
			rpcError(w, request.ID, -32600, "Unknown session", 404)
			return
		}
	}
	if request.Method == "notifications/initialized" {
		w.WriteHeader(202)
		return
	}
	var result any
	switch request.Method {
	case "initialize":
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			rpcError(w, request.ID, -32603, "Internal error", 500)
			return
		}
		sid := hex.EncodeToString(id)
		s.mu.Lock()
		s.sessions[sid] = true
		s.mu.Unlock()
		w.Header().Set("MCP-Session-Id", sid)
		result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]any{"name": "trilha-orders", "version": "1.0.0"}}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": []any{s.tool}}
	case "tools/call":
		var params struct {
			Name      string `json:"name"`
			Arguments struct {
				OrderID string `json:"orderId"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.Name != "findOrder" {
			rpcError(w, request.ID, -32602, "Invalid params", 200)
			return
		}
		value := domain.QueryOrder(params.Arguments.OrderID)
		result = struct {
			Content    []any           `json:"content"`
			Structured json.RawMessage `json:"structuredContent"`
			IsError    bool            `json:"isError"`
		}{[]any{map[string]string{"type": "text", "text": value}}, json.RawMessage(value), false}
	default:
		rpcError(w, request.ID, -32601, "Method not found", 200)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
}
func rpcError(w http.ResponseWriter, id json.RawMessage, code int, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}
