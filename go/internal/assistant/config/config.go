package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	LabRoot      string
	Host         string
	Port         int
	MCPPort      int
	MCPURL       string
	BaseURL      string
	APIKey       string
	RoutingModel string
	ExpertModel  string
	ContractPath string
	AssetsDir    string
	ORTLibrary   string
	ModelTimeout time.Duration
	ToolTimeout  time.Duration
}

type Contract struct {
	Prompts        map[string]string          `json:"prompts"`
	ExpertPrefixes map[string]string          `json:"expertPrefixes"`
	Tools          map[string]json.RawMessage `json:"tools"`
	Rag            map[string]any             `json:"rag"`
	Supervisor     struct {
		Agents   string            `json:"agents"`
		AgentIDs map[string]string `json:"agentIds"`
		Context  string            `json:"supervisorContext"`
	} `json:"supervisor"`
	Limits     map[string]int `json:"limits"`
	Generation map[string]any `json:"generation"`
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
func number(name string, fallback int) int {
	value, err := strconv.Atoi(env(name, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

func Load() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	root := env("LAB_ROOT", cwd)
	if _, err := os.Stat(filepath.Join(root, "knowledge")); err != nil {
		root = filepath.Dir(root)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Config{}, err
	}
	assets := env("EMBEDDING_ASSETS_DIR", filepath.Join(root, "comparison", "assets"))
	cfg := Config{LabRoot: root, Host: env("HOST", "127.0.0.1"), Port: number("PORT", 8084), MCPPort: number("MCP_SERVER_PORT", 8085), MCPURL: env("MCP_URL", "http://127.0.0.1:8081/orders"), BaseURL: env("OCI_BASE_URL", "http://127.0.0.1:9000/v1"), APIKey: env("OCI_API_KEY", "local-replay"), RoutingModel: env("OCI_ROUTING_MODEL", "trilha-replay"), ExpertModel: env("OCI_EXPERT_MODEL", "trilha-replay"), ContractPath: env("CONTRACT_PATH", filepath.Join(root, "comparison", "contract.json")), AssetsDir: assets, ORTLibrary: env("ONNXRUNTIME_SHARED_LIBRARY_PATH", env("ORT_LIBRARY_PATH", "")), ModelTimeout: 60 * time.Second, ToolTimeout: 10 * time.Second}
	if cfg.Port < 1 || cfg.Port > 65535 || cfg.MCPPort < 1 || cfg.MCPPort > 65535 {
		return Config{}, errors.New("porta inválida")
	}
	return cfg, nil
}

func ReadContract(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}
	var result Contract
	err = json.Unmarshal(data, &result)
	if err == nil && len(result.Prompts) == 0 {
		err = errors.New("contrato sem prompts")
	}
	return result, err
}
