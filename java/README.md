# Trilha em Java com Helidon

Este laboratório combina interfaces declarativas do Helidon AI com classificação, especialistas, tools, RAG, workflow, supervisor e MCP. O HTTP usa Helidon SE e virtual threads; o embedding MiniLM executa localmente.

## Pré-requisitos e build

Use JDK 21 de 64 bits e Maven 3.9.9 ou superior. As dependências fixam Helidon 4.5.6, MCP 1.2.0 e LangChain4j 1.18.1, incluindo módulos beta gerenciados pelo BOM.

A partir da raiz do repositório, execute `cd java`. Os comandos desta página partem desse diretório:

```shell
java -version
mvn -version
mvn -B -DskipTests package
```

Mantenha `assistant/target/libs` e `mcp-server/target/libs` junto dos respectivos JARs.

## Preparar os índices RAG

Os arquivos `knowledge/orders-embeddings.json` e `knowledge/returns-embeddings.json` já preparados podem ser reutilizados. Em um checkout sem esses arquivos, ou após mudar as políticas, execute a ingestão existente.

Linux:

```bash
java -cp 'assistant/target/assistant.jar:assistant/target/libs/*' lab.trilha.rag.IngestPolicies
```

PowerShell:

```powershell
java -cp 'assistant/target/assistant.jar;assistant/target/libs/*' lab.trilha.rag.IngestPolicies
```

A ingestão lê os dois Markdown de `knowledge`. Não chama a OCI. Para a comparação, todas as linguagens devem ler esses mesmos índices e políticas.

## Iniciar o servidor MCP

Em um primeiro terminal, em `java`:

```shell
java -jar mcp-server/target/mcp-server.jar
```

Mantenha-o aberto. A ferramenta `findOrder` fica em `http://127.0.0.1:8081/orders` e não exige credencial OCI. Python e Go também têm servidores MCP próprios, descritos em seus READMEs. Usar este servidor como dependência comum dos três assistentes é uma alternativa de integração; registre essa escolha ao avaliar a rota `/api/v1/mcp`.

## Configurar OCI e iniciar o assistente

Configure as quatro variáveis no terminal do **assistente**, preservando a mesma região, chave autorizada e modelos do ensaio. A URL abaixo é o exemplo de Chicago; troque-a quando usar outra região.

Linux/Bash:

```bash
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key GenAI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<identificador do modelo>'
export OCI_EXPERT_MODEL='<identificador do modelo>'
java -jar assistant/target/assistant.jar
```

Windows/PowerShell 7:

```powershell
$env:OCI_BASE_URL = 'https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
$env:OCI_API_KEY = Read-Host -MaskInput 'Segredo da API key GenAI'
$env:OCI_ROUTING_MODEL = '<identificador do modelo>'
$env:OCI_EXPERT_MODEL = '<identificador do modelo>'
java -jar assistant/target/assistant.jar
```

A credencial é a [API key do Generative AI](../README.md#autenticação-na-oci). A aplicação acrescenta `/chat/completions` à URL base. Preencha os dois identificadores com modelos disponíveis na região e autorizados para a chave.

O bind padrão é `127.0.0.1:8080`. Para JMeter em outra máquina da rede privada, inicie com `java '-Dserver.host=0.0.0.0' -jar assistant/target/assistant.jar` e permita a porta 8080 na rede/firewall da VM somente para o gerador. Para mudar porta ou URI MCP, use propriedades de execução:

```shell
java -Dserver.port=8080 -Dlangchain4j.mcp-clients.order-server.uri=http://127.0.0.1:8081/orders -jar assistant/target/assistant.jar
```

No PowerShell, coloque cada argumento `-D...` entre aspas simples.

## Conferir manualmente

```bash
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/api/v1/runtime
curl -sS http://127.0.0.1:8080/api/v1/workflow \
  -H 'Content-Type: application/json' \
  -d '{"message":"Qual o status do pedido TR-1001?","summary":""}'
```

No Windows, use `curl.exe`. `/health` verifica o processo; a chamada de negócio é que confere acesso ao modelo. `virtualThread: true` descreve a thread que atendeu o diagnóstico, sem caracterizar todos os executores internos.

| Rota POST | Fluxo |
|---|---|
| `/api/v1/direct` | Serviço de trocas, RAG e tool; resumo retornado vazio |
| `/api/v1/workflow` | Classificação → especialista selecionado → resumo |
| `/api/v1/supervisor` | Planejador → especialistas → síntese → resumo |
| `/api/v1/mcp` | Especialista de pedidos → tool MCP → resumo |

`GET /api/v1/metrics` fornece contadores cumulativos. Envie o resumo anterior em `summary` para continuar uma conversa; vazio inicia outra. Não há histórico global de sessões.

## Ler o código

Em `assistant/src/main/java/lab/trilha`, comece por `ai/PolicyService`, `OrderAgent`/`ReturnAgent` e `SupportWorkflow`. `@Ai.Service` e `@Ai.Agent` compõem modelos, retrievers, tools e guardrails. `@SequenceAgent`, `@ConditionalAgent`, `@ActivationCondition` e `@Output` expressam o workflow. `@SupervisorAgent` e `@SupervisorRequest` montam a delegação dinâmica, limitada a três invocações de subagentes.

`OrderCatalog` no módulo `domain` aplica a regra das tools locais e MCP; `OrderMcpServer` expõe a consulta remota. O limite de subagentes não equivale a três completions. Os guardrails são exemplos determinísticos, sem promessa de proteção geral contra prompt injection.

Para carga, use o [JMX único](../jmeter/agentic.jmx) e o [procedimento manual](../docs/COMPARACAO_MANUAL.md). No benchmark de `/api/v1/workflow`, as ferramentas são locais e a coleta de CPU/memória acompanha somente o PID de `assistant.jar`. O processo `mcp-server.jar` fica fora dessa coleta; o cliente MCP do assistente continua inicializado e faz verificações periódicas. Dois processos na mesma VM ainda compartilham os recursos dela.

O tempo HTTP inclui as chamadas à OCI. Para medir `/api/v1/mcp`, faça outro ensaio e registre os PIDs do assistente e do servidor separadamente: nessa rota, a latência também inclui a chamada MCP. Os [contratos HTTP e de comportamento](contracts/BEHAVIOR.md) detalham cada cenário.

Fontes: [Helidon 4](https://helidon.io/docs/v4/se/ai/langchain4j), [HOL de referência](https://github.com/helidon-io/helidon-labs/tree/3d8f16caaa16958debe07cef681c819c87f83d5d/hols/langchain4j-agentic) e [extensão MCP 1.2.0](https://github.com/helidon-io/helidon-mcp/tree/1.2.0).
