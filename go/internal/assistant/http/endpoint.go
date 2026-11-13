package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"lab.trilha/go/internal/assistant/ai"
	"lab.trilha/go/internal/assistant/guardrail"
	"lab.trilha/go/internal/assistant/metrics"
	"log"
	"mime"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

type Handler struct {
	Agents       *ai.Agents
	Metrics      *metrics.Counters
	ProviderMode string
}

func New(agents *ai.Agents, counters *metrics.Counters, base string) *Handler {
	mode := "remote"
	if parsed, err := url.Parse(base); err == nil && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost") {
		mode = "local"
	}
	return &Handler{agents, counters, mode}
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		writeJSON(w, map[string]string{"status": "UP"})
		return
	case "/api/v1/runtime":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		writeJSON(w, map[string]any{"language": "go", "runtimeVersion": runtime.Version(), "virtualThread": false, "providerMode": h.ProviderMode, "concurrency": "goroutines", "workers": 1})
		return
	case "/api/v1/metrics":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		writeJSON(w, h.Metrics.Snapshot())
		return
	}
	route := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if route != "workflow" && route != "direct" && route != "supervisor" && route != "mcp" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(405)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "Content-Type deve ser application/json.", 415)
		return
	}
	var raw struct {
		Message *string `json:"message"`
		Summary *string `json:"summary"`
	}
	decoder := json.NewDecoder(r.Body)
	if err = decoder.Decode(&raw); err != nil {
		http.Error(w, "JSON inválido.", 400)
		return
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "JSON inválido.", 400)
		return
	}
	if raw.Message == nil {
		http.Error(w, "message deve conter 1 a 2000 pontos de código Unicode e summary até 1500.", 400)
		return
	}
	summary := ""
	if raw.Summary != nil {
		summary = *raw.Summary
	}
	if err = guardrail.ValidateRequest(*raw.Message, summary); err != nil {
		var blocked *guardrail.Failure
		if errors.As(err, &blocked) {
			http.Error(w, "Entrada bloqueada pelo guardrail.", 422)
		} else {
			http.Error(w, err.Error(), 400)
		}
		return
	}
	request := ai.Request{Message: *raw.Message, Summary: summary}
	var response ai.Response
	switch route {
	case "workflow":
		response, err = h.Agents.Workflow(r.Context(), request)
	case "direct":
		response, err = h.Agents.Direct(r.Context(), request)
	case "mcp":
		response, err = h.Agents.MCPWorkflow(r.Context(), request)
	case "supervisor":
		response, err = h.Agents.SupervisorWorkflow(r.Context(), request)
	}
	if err != nil {
		var blocked *guardrail.Failure
		if errors.As(err, &blocked) {
			http.Error(w, "Conteúdo bloqueado pelo guardrail.", 422)
			return
		}
		var provider *ai.ProviderError
		if errors.As(err, &provider) {
			log.Printf("Falha no provedor ou fluxo: tipo=ProviderError; httpStatus=%d", provider.Status)
		} else {
			log.Printf("Falha no provedor ou fluxo: tipo=%T", err)
		}
		http.Error(w, "Não foi possível concluir a consulta ao provedor ou à ferramenta.", 502)
		return
	}
	writeJSON(w, response)
}
