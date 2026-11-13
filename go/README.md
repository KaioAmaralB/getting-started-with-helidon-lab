# Trilha em Go

Este laboratório usa net/http, goroutines e ONNX Runtime via CGO. Os agentes, workflow, supervisor, RAG, ferramentas e guardrails seguem os contratos do laboratório Java. O atendimento e o servidor MCP executam em Go.

## Preparar e compilar

Use Go compatível com `go.mod` (Go 1.23 ou superior), um compilador C para CGO e ONNX Runtime **1.20.0 CPU**, todos para a mesma arquitetura. O [roteiro Oracle Linux](../docs/BENCHMARK_ORACLE_LINUX.md) fixa as versões do seu exemplo de instalação.

Instale Go e GCC no sistema e disponibilize-os no `PATH`. Ferramentas portáteis, binários e caches locais não fazem parte do repositório. A partir da raiz, execute `cd go`; os comandos desta página partem desse diretório.

**Windows/PowerShell 7**, com GCC x64 compatível com CGO:

```powershell
go version
gcc --version
$env:CC = 'gcc'
$env:CGO_ENABLED = '1'
go mod download
go build -o bin/assistant.exe ./cmd/assistant
go build -o bin/mcp-server.exe ./cmd/mcp-server
```

**Windows/Git Bash**, com os mesmos Go e GCC Windows no `PATH`:

```bash
go version
gcc --version
export CC=gcc
export CGO_ENABLED=1
go mod download
go build -o bin/assistant.exe ./cmd/assistant
go build -o bin/mcp-server.exe ./cmd/mcp-server
```

**Linux**, com Go e GCC instalados:

```bash
go version
gcc --version
export CC=gcc
export CGO_ENABLED=1
go mod download
go build -o bin/assistant ./cmd/assistant
go build -o bin/mcp-server ./cmd/mcp-server
```

Compile para o sistema de destino: o `.exe` Windows não executa no Linux. Prepare o modelo, o tokenizer e a biblioteca nativa pelos [comandos de extração](../comparison/assets/README.md), a partir dos mesmos JARs utilizados pelo Java.

## Iniciar MCP

Inicie o servidor MCP Go em outro terminal, no diretório `go`. Ele consulta o catálogo fictício e lê o schema em `comparison/contract.json`; não carrega os índices RAG nem chama a OCI.

**Linux/Bash:**

```bash
CONTRACT_PATH="$(cd .. && pwd)/comparison/contract.json" \
HOST=127.0.0.1 MCP_SERVER_PORT=8085 ./bin/mcp-server
```

**Windows/Git Bash:**

```bash
CONTRACT_PATH="$(cygpath -am ..)/comparison/contract.json" \
HOST=127.0.0.1 MCP_SERVER_PORT=8085 ./bin/mcp-server.exe
```

**Windows/PowerShell 7:**

```powershell
$env:CONTRACT_PATH = (Resolve-Path '../comparison/contract.json').Path
$env:HOST = '127.0.0.1'
$env:MCP_SERVER_PORT = '8085'
./bin/mcp-server.exe
```

Mantenha-o aberto em `http://127.0.0.1:8085/orders` antes de iniciar o assistente. Esses comandos não precisam de `LAB_ROOT`: o caminho do contrato já está explícito.

## Configurar caminhos, OCI e executar

O assistente procura os índices em `LAB_ROOT/knowledge`. Por isso, `LAB_ROOT` aponta para a pasta `java`, onde estão as políticas e os vetores compartilhados. Trata-se de uma pasta de dados: esse caminho não executa Java nem conecta o assistente ao MCP Java. O contrato e os assets ONNX ficam em `comparison` e recebem caminhos próprios nos comandos abaixo.

**Windows/Git Bash**, em `go`: use caminhos Windows e a DLL. O Git Bash executa o binário Windows, mesmo aceitando comandos Bash.

```bash
LAB_DIR="$(cygpath -am ..)"
export LAB_ROOT="$LAB_DIR/java"
export CONTRACT_PATH="$LAB_DIR/comparison/contract.json"
export EMBEDDING_ASSETS_DIR="$LAB_DIR/comparison/assets"
export ONNXRUNTIME_SHARED_LIBRARY_PATH="$LAB_DIR/comparison/.tools/onnxruntime/onnxruntime.dll"
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key GenAI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<mesmo modelo usado no Java>'
export OCI_EXPERT_MODEL='<mesmo modelo usado no Java>'
export MCP_URL='http://127.0.0.1:8085/orders'
./bin/assistant.exe
```

**Linux/Bash**, em `go`: esta opção usa a biblioteca `.so` e um executável compilado para Linux.

```bash
LAB_DIR="$(cd .. && pwd)"
export LAB_ROOT="$LAB_DIR/java"
export CONTRACT_PATH="$LAB_DIR/comparison/contract.json"
export EMBEDDING_ASSETS_DIR="$LAB_DIR/comparison/assets"
export ONNXRUNTIME_SHARED_LIBRARY_PATH="$LAB_DIR/comparison/.tools/onnxruntime/libonnxruntime.so"
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key GenAI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<mesmo modelo usado no Java>'
export OCI_EXPERT_MODEL='<mesmo modelo usado no Java>'
export MCP_URL='http://127.0.0.1:8085/orders'
./bin/assistant
```

Windows/PowerShell 7:

```powershell
$labRoot = (Resolve-Path '..').Path
$env:LAB_ROOT = Join-Path $labRoot 'java'
$env:CONTRACT_PATH = Join-Path $labRoot 'comparison/contract.json'
$env:EMBEDDING_ASSETS_DIR = Join-Path $labRoot 'comparison/assets'
$env:ONNXRUNTIME_SHARED_LIBRARY_PATH = Join-Path $labRoot 'comparison/.tools/onnxruntime/onnxruntime.dll'
$env:OCI_BASE_URL = 'https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
$env:OCI_API_KEY = Read-Host -MaskInput 'Segredo da API key GenAI'
$env:OCI_ROUTING_MODEL = '<mesmo modelo usado no Java>'
$env:OCI_EXPERT_MODEL = '<mesmo modelo usado no Java>'
$env:MCP_URL = 'http://127.0.0.1:8085/orders'
./bin/assistant.exe
```

A [autenticação OCI](../README.md#autenticação-na-oci) usa o segredo da API key do Generative AI. O cliente acrescenta `/chat/completions` à URL base. `.env` e `~/.oci/config` não são carregados automaticamente.

O bind padrão é `127.0.0.1:8084`. Para JMeter em outra máquina da rede privada, configure `HOST=0.0.0.0` e permita a porta 8084 na rede/firewall da VM somente para o gerador.

Se aparecer `Inicialização encerrada: *fmt.wrapError`, o binário está exibindo apenas o tipo do erro. Confira os caminhos de contrato, índices, tokenizer/modelo, biblioteca ONNX correspondente ao sistema e disponibilidade do MCP. No Git Bash do Windows, não use `libonnxruntime.so`: use o bloco Windows/Git Bash acima.

## Conferir manualmente

```bash
curl -sS http://127.0.0.1:8084/health
curl -sS http://127.0.0.1:8084/api/v1/runtime
curl -sS http://127.0.0.1:8084/api/v1/workflow \
  -H 'Content-Type: application/json' \
  -d '{"message":"Qual o status do pedido TR-1001?","summary":""}'
```

No Windows, use `curl.exe`. Os POSTs `/api/v1/direct`, `/workflow`, `/supervisor` e `/mcp` têm o mesmo contrato `message`/`summary` → `answer`/`summary` do Java. `GET /api/v1/metrics` expõe os contadores de modelo.

Use a porta 8084 no [JMX único](../jmeter/agentic.jmx). No ensaio de `/api/v1/workflow`, colete CPU e memória somente do PID de `assistant`; a ferramenta dessa rota é local. O servidor MCP precisa estar disponível para a inicialização e as verificações do cliente, mas seu PID não integra essa coleta.

A rota `/api/v1/mcp` é outro ensaio: o tempo HTTP inclui o servidor remoto. Registre separadamente os recursos de cada processo e a linguagem do servidor usado. O [guia manual](../docs/COMPARACAO_MANUAL.md) detalha a carga e a coleta.

## Organização e opções de MCP

`cmd/assistant` e `cmd/mcp-server` são os processos. Em `internal/assistant` ficam `ai`, `config`, `guardrail`, `http`, `metrics` e `rag`; `internal/domain` mantém o catálogo; `internal/mcp_server` mantém cliente e servidor MCP.

O valor padrão de `MCP_URL` no código é `http://127.0.0.1:8081/orders`. Os comandos deste guia o substituem pela porta 8085 do MCP Go. Para usar um servidor comum, inicie, por exemplo, o [MCP Java](../java/README.md#iniciar-o-servidor-mcp) e defina `MCP_URL` com a URL dele antes de iniciar o assistente. Registre essa escolha nos resultados de `/api/v1/mcp`; não misture as duas topologias no mesmo conjunto de medições.

`HOST` e `PORT` controlam o bind do assistente; `MCP_SERVER_PORT` tem padrão 8085. Os timeouts são 60 s no modelo e 10 s na operação MCP. Os pools HTTP e a sessão ONNX são reutilizados. A quantização, tokenizer e corpus devem permanecer os mesmos nas três linguagens.

Fontes: [Go net/http](https://pkg.go.dev/net/http), [binding ONNX v1.16.0](https://github.com/yalue/onnxruntime_go/tree/v1.16.0) e [ONNX Runtime 1.20.0](https://github.com/microsoft/onnxruntime/releases/tag/v1.20.0).
