package main

import (
	"fmt"
	"lab.trilha/go/internal/assistant/config"
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
	handler, err := mcp.NewServer(contract)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.MCPPort), Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("MCP Go pronto em http://%s:%d/orders", cfg.Host, cfg.MCPPort)
	return server.ListenAndServe()
}
func main() {
	if err := run(); err != nil {
		log.Fatalf("Inicialização encerrada: %T", err)
	}
}
