# Trilha em Python

A aplicação usa FastAPI/ASGI, asyncio, HTTPX e MiniLM ONNX no próprio processo. Os quatro fluxos, prompts, dados, tools e guardrails correspondem ao laboratório Java.

## Preparar

Use **Python 3.13 x64**. O ambiente de referência usa 3.13.14, NumPy 2.1.3 e ONNX Runtime 1.20.0. Crie um ambiente limpo; não execute `venv` sobre uma pasta de outra versão. As extensões nativas fixadas no lock precisam corresponder ao interpretador.

A partir da raiz do repositório, execute `cd python`. Os comandos abaixo partem desse diretório. No Windows, o Python Launcher deve ter o Python 3.13 disponível; alternativamente, use o caminho completo do executável 3.13.

```powershell
py -3.13 -m venv .venv
.venv/Scripts/python.exe -m pip install -r requirements.lock
```

Linux:

```bash
python3.13 -m venv .venv
.venv/bin/python -m pip install -r requirements.lock
```

Se a `.venv` atual já estiver preparada corretamente, reutilize-a. As dependências diretas e transitivas permanecem fixadas; não atualize apenas uma linguagem durante um ensaio.

No Git Bash do Windows, o executável continua em `Scripts`, pois o ambiente é Windows. Depois de preparar a `.venv`, confira:

```bash
./.venv/Scripts/python.exe --version
```

O resultado deve ser Python 3.13.x. Se precisar criar o ambiente pelo Git Bash sem Python Launcher, confira `python --version` e use `python -m venv .venv` somente quando esse interpretador for 3.13 e a pasta de destino for nova. A inicialização com OCI está na seção abaixo.

Confira os [recursos compartilhados](../comparison/README.md): contrato, tokenizer, modelo ONNX e os dois índices de `../java/knowledge`. Os arquivos em `java/knowledge` são dados compartilhados: lê-los não executa código Java.

## Iniciar MCP

Inicie o servidor MCP Python em outro terminal, no diretório `python`. Ele usa o contrato em `comparison/contract.json` e o catálogo fictício. Não carrega RAG nem chama a OCI.

**Linux/Bash:**

```bash
LAB_ROOT="$(cd .. && pwd)" HOST=127.0.0.1 MCP_SERVER_PORT=8083 \
.venv/bin/python -m trilha.mcp_server
```

**Windows/PowerShell 7:**

```powershell
$env:LAB_ROOT = (Resolve-Path '..').Path
$env:HOST = '127.0.0.1'
$env:MCP_SERVER_PORT = '8083'
.venv/Scripts/python.exe -m trilha.mcp_server
```

**Windows/Git Bash:**

```bash
unset LAB_ROOT
HOST=127.0.0.1 MCP_SERVER_PORT=8083 \
./.venv/Scripts/python.exe -m trilha.mcp_server
```

Mantenha-o aberto em `http://127.0.0.1:8083/orders` antes de iniciar o assistente. Aqui, `LAB_ROOT` é a raiz do repositório, para localizar o contrato; se não for definido, o pacote infere essa raiz pelo próprio arquivo.

## Configurar OCI e executar

No terminal do assistente, em `python`, Linux/Bash:

```bash
export LAB_ROOT="$(cd .. && pwd)"
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key GenAI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<mesmo modelo usado no Java>'
export OCI_EXPERT_MODEL='<mesmo modelo usado no Java>'
export MCP_URL='http://127.0.0.1:8083/orders'
.venv/bin/python -m trilha.assistant
```

Windows/PowerShell 7:

```powershell
$env:LAB_ROOT = (Resolve-Path '..').Path
$env:OCI_BASE_URL = 'https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
$env:OCI_API_KEY = Read-Host -MaskInput 'Segredo da API key GenAI'
$env:OCI_ROUTING_MODEL = '<mesmo modelo usado no Java>'
$env:OCI_EXPERT_MODEL = '<mesmo modelo usado no Java>'
$env:MCP_URL = 'http://127.0.0.1:8083/orders'
.venv/Scripts/python.exe -m trilha.assistant
```

Windows/Git Bash, no terminal do assistente, depois de iniciar o MCP:

```bash
unset LAB_ROOT
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key GenAI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<mesmo modelo usado no Java>'
export OCI_EXPERT_MODEL='<mesmo modelo usado no Java>'
export MCP_URL='http://127.0.0.1:8083/orders'
./.venv/Scripts/python.exe -m trilha.assistant
```

Nesse shell, `unset LAB_ROOT` deixa o pacote localizar automaticamente a raiz do repositório, sem herdar um caminho Linux. Use a [API key regional do Generative AI](../README.md#autenticação-na-oci), não `[DEFAULT]` ou PEM. `OCI_BASE_URL` termina em `/actions/v1`; o cliente acrescenta `/chat/completions`. Não há carregamento automático de `.env`.

O bind padrão é `127.0.0.1:8082`. Para JMeter em outra máquina da rede privada, configure `HOST=0.0.0.0` antes de executar e permita a porta 8082 na rede/firewall da VM somente para o gerador.

## Conferir e testar no JMeter

```bash
curl -sS http://127.0.0.1:8082/health
curl -sS http://127.0.0.1:8082/api/v1/runtime
curl -sS http://127.0.0.1:8082/api/v1/workflow \
  -H 'Content-Type: application/json' \
  -d '{"message":"Qual o status do pedido TR-1001?","summary":""}'
```

No Windows, use `curl.exe`. A resposta de negócio contém `answer` e `summary`. `POST /api/v1/direct` atende trocas, `/workflow` classifica e roteia, `/supervisor` delega pelo modelo e `/mcp` consulta pedidos por tool remota. `GET /api/v1/metrics` expõe os contadores de modelo.

Abra o [JMX único](../jmeter/agentic.jmx) e use a porta 8082. No ensaio de `/api/v1/workflow`, a ferramenta é local: colete CPU e memória somente do PID do assistente Uvicorn, sem incluir o processo MCP. O assistente mantém a conexão MCP e suas verificações periódicas mesmo nessa rota.

Meça `/api/v1/mcp` separadamente, pois seu tempo HTTP inclui a chamada ao servidor remoto. Identifique o servidor escolhido e colete os recursos dos dois processos em separado, conforme o [guia manual](../docs/COMPARACAO_MANUAL.md).

## Opções e estrutura

| Variável | Padrão |
|---|---|
| `LAB_ROOT` | Raiz do repositório inferida pelo pacote |
| `HOST` / `PORT` | `127.0.0.1` / `8082` |
| `MCP_URL` | `http://127.0.0.1:8081/orders` |
| `MCP_SERVER_PORT` | `8083` |
| `RAG_WORKERS` | `256` |

`trilha/assistant` separa `ai`, `guardrail`, `http`, `metrics` e `rag`; `trilha/domain` contém o catálogo; `trilha/mcp_server` contém cliente e servidor MCP.

O atendimento usa um processo Uvicorn. A inferência nativa sai do event loop por um pool de até `RAG_WORKERS` threads. Para os ensaios propostos, mantenha esse valor pelo menos igual à concorrência máxima e registre-o. Os timeouts são 60 s por completion e 10 s no MCP; os clientes reutilizam conexões e negociam HTTP/2 quando disponível.

O padrão de `MCP_URL` no código é a porta 8081; os comandos deste guia usam explicitamente a porta 8083 do MCP Python. Se quiser manter um servidor comum entre os assistentes, inicie, por exemplo, o [MCP Java](../java/README.md#iniciar-o-servidor-mcp) e configure sua URL em `MCP_URL`. Registre essa topologia separadamente nos ensaios de `/api/v1/mcp`.

Compatibilidade com Python 3.13: [NumPy 2.1](https://numpy.org/doc/2.1/release/2.1.0-notes.html) e [binários do ONNX Runtime 1.20.0](https://pypi.org/project/onnxruntime/1.20.0/). O modelo e a versão ONNX permanecem iguais aos do Java e Go. A adequação do embedding ao domínio e os guardrails exigem avaliação própria; o exemplo não demonstra segurança geral nem qualidade de ranking.
