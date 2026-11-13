# Contrato de comportamento das implementações

Este arquivo complementa o OpenAPI e descreve o comportamento compartilhado por Java, Python e Go. Java é a referência dos prompts, dos dados e das políticas. A comparação deve preservar o trabalho de negócio e as interações com dependências, respeitando os modelos de concorrência de cada runtime.

## Cenários separados

| Rota | Etapas | Dependência de negócio |
|---|---|---|
| `/api/v1/direct` | AI Service de trocas; resposta sem atualizar resumo | RAG returns; tool local se solicitada |
| `/api/v1/workflow` | Classificação → exatamente um especialista → resumo → composição do DTO | RAG do domínio; tool local |
| `/api/v1/supervisor` | Planejar/delegar até três vezes → síntese → resumo → DTO | Os mesmos especialistas e tools locais |
| `/api/v1/mcp` | Especialista de pedidos → resumo → DTO | RAG orders; tool via outro processo MCP |

Uma solicitação de tool pelo modelo e a completion que interpreta seu resultado são rodadas distintas. A quantidade efetiva de chamadas depende do fluxo, das decisões do modelo e das falhas observadas; registre os contadores durante o ensaio, em vez de assumir uma quantidade fixa por requisição HTTP.

Os prompts canônicos estão em `assistant/src/main/java/lab/trilha/ai`: `Prompts.EXPERT`, `PolicyService`, `IntentAgent`, `SummaryAgent` e `SupportSupervisor`. O prompt do planejador vem do LangChain4j `PlannerAgent` na versão fixada e deve ser incluído no inventário de equivalência do supervisor. Congele os textos efetivamente enviados, incluindo instruções de schema e contexto RAG; usar a mesma descrição de negócio em bibliotecas diferentes não garante prompts iguais.

## Escopo do benchmark e MCP

O benchmark do artigo usa `/api/v1/workflow`, com ferramentas locais. A coleta de CPU e memória considera somente o PID do assistente em cada linguagem, incluindo seu HTTP, orquestração, RAG e clientes de serviços. O servidor MCP é um processo separado e não integra essa coleta. Seu cliente continua inicializado no assistente, com verificações periódicas; processos na mesma VM compartilham recursos.

Java, Python e Go têm servidores MCP próprios. Os guias usam o servidor da respectiva linguagem. Apontar todos os clientes para um servidor comum é uma alternativa para manter a dependência externa fixa.

A rota `/api/v1/mcp` deve ser medida em separado: seu tempo inclui comunicação e execução no servidor MCP. Identifique a linguagem e a localização desse servidor e colete os dois PIDs separadamente. Em qualquer rota, o tempo HTTP também inclui a espera pelos modelos na OCI.

As políticas e os índices em `java/knowledge` são dados compartilhados, não execução de um componente Java por Python ou Go. O `LAB_ROOT` do assistente Go aponta para `java` para localizar esses índices. O MCP Go não os usa e precisa apenas do caminho do contrato compartilhado.

## Oráculo de negócio

| Entrada ou fato | Resultado esperado |
|---|---|
| `TR-1001` | Item barraca, `IN_TRANSIT`, estimativa informativa de até 3 dias úteis |
| `TR-1002` | Item mochila, `DELIVERED`, entregue |
| Outro ID no formato `TR-` + quatro dígitos ASCII | Tool devolve `{"error":"ORDER_NOT_ALLOWED"}`; sem status inventado |
| ID inválido ou ausente passado diretamente à tool | Tool devolve `{"error":"INVALID_ORDER_ID"}` |
| Pergunta sem ID | Modelo deve pedir o identificador se a consulta depender do pedido; não inventar ID |
| Orientação de troca | `POL-TROCA-v1`: até 30 dias após a entrega, item sem uso, avaliação e autorização humanas |
| Consulta de entrega | `POL-ENTREGA-v1`: status vem da tool e estimativas são informativas |
| Pedido para executar estorno/troca/alteração | Não existe tool de escrita; não confirmar execução |

O catálogo e a serialização canônica ficam em `domain`. A tool local e a tool MCP têm nome `findOrder` e argumento string `orderId`. O MCP também devolve `structuredContent`; compare campos e valores JSON, sem usar ordem das propriedades como critério de negócio. Para igualar prompts, confira também a representação textual que cada cliente efetivamente envia ao modelo.

## Estado, validação e contexto

Cada chamada cria um escopo de execução novo. `summary` vazio indica conversa nova; outro valor é contexto fornecido pelo cliente. Não há cache de respostas, histórico global de usuários ou memória persistente. Não use resumo como prova de autorização ou de status.

`message` tem até 2000 pontos de código Unicode; `summary`, até 1500. Entrada vazia ou somente espaços é inválida. Marcadores PEM explícitos são rejeitados antes da chamada remota. As regras de guardrail devem ser reproduzidas com o mesmo alcance para medir trabalho equivalente; o teste de qualidade deve permanecer separado da carga.

As políticas Markdown e seus embeddings carregados são parte da versão do workload. Use os mesmos vetores e segmentos, a mesma similaridade, limites e contexto final nas outras implementações. Os arquivos `knowledge/*-embeddings.json` são JSON serializado pelo LangChain4j e não exigem um banco vetorial. Um adaptador em outra linguagem pode lê-los; antes de medir, compare os resultados e textos recuperados para o mesmo conjunto de consultas. Mudar embedding, segmentação ou limiar caracteriza outro experimento.

Métricas de modelo são cumulativas e agregadas por processo. Conte falhas, solicitações de tools e todas as rodadas, comparando leituras anteriores e posteriores à carga. Identifique a rota e os modelos usados em cada ensaio; o [protocolo](../../docs/COMPARACAO_MANUAL.md) define as condições de coleta e de relatório.
