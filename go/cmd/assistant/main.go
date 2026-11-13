package main

import (
	"context"
	"fmt"
	"lab.trilha/go/internal/assistant/ai"
	"lab.trilha/go/internal/assistant/config"
	httpapi "lab.trilha/go/internal/assistant/http"
	"lab.trilha/go/internal/assistant/metrics"
	"lab.trilha/go/internal/assistant/rag"
	mcp "lab.trilha/go/internal/mcp_server"
	"log"
	"net/http"
	"time"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	contract, err := config.ReadContract(cfg.ContractPath)
	if err != nil {
		return err
	}
	retriever, err := rag.NewRetriever(cfg.LabRoot, cfg.AssetsDir, cfg.ORTLibrary)
	if err != nil {
		return err
	}
	defer retriever.Close()
	client, err := mcp.NewClient(context.Background(), cfg.MCPURL, cfg.ToolTimeout)
	if err != nil {
		return err
	}
	defer client.Close()
	counters := &metrics.Counters{}
	model := ai.NewClient(cfg.BaseURL, cfg.APIKey, cfg.ModelTimeout, counters)
	if len(contract.Generation) > 0 {
		model.Parameters = contract.Generation
	}
	agents := &ai.Agents{Contract: contract, Model: model, Retrieval: retriever, Remote: client, RoutingModel: cfg.RoutingModel, ExpertModel: cfg.ExpertModel}
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Handler: httpapi.New(agents, counters, cfg.BaseURL), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("Trilha Go pronta em http://%s:%d", cfg.Host, cfg.Port)
	return server.ListenAndServe()
}
func main() {
	if err := run(); err != nil {
		log.Fatalf("Inicialização encerrada: %T", err)
	}
}
