# Comparação manual: Java, Python e Go

O cenário principal compara os assistentes Java, Python e Go pela rota `POST /api/v1/workflow`, com uma VM por linguagem. Os comandos partem da raiz do repositório, salvo indicação contrária. O [roteiro Oracle Linux](BENCHMARK_ORACLE_LINUX.md) detalha a preparação das VMs.

CPU e memória são coletadas do **processo do assistente**. Essa medida inclui HTTP, orquestração, RAG local, bibliotecas nativas e o cliente OCI. O tempo de resposta observado pelo JMeter inclui também rede e espera pelo modelo. A latência e as quotas da OCI podem dominar esse tempo; os resultados descrevem as pilhas nas condições registradas e não isolam o custo da linguagem.

O procedimento usa os READMEs, um único JMX e ferramentas do Linux. Os valores de concorrência e repetição sugeridos abaixo são pontos de partida, não resultados medidos.

## 1. Fixar o ambiente e o trabalho

Use três VMs com a mesma arquitetura, sistema operacional e recursos disponíveis, uma para cada linguagem. Execute a carga em uma aplicação por vez. Mantenha o JMeter em outra máquina e use o mesmo caminho de rede até as três VMs. Observe também CPU e memória do gerador, pois ele pode limitar o ensaio.

Registre:

- Versões de Java, Python e Go, flags, bibliotecas e número de processos/workers.
- Região, URL, modelos OCI e parâmetros de geração.
- Políticas, índices, modelo ONNX, tokenizer, mensagem e resumo inicial.
- Topologia MCP, aquecimento, carga e recursos disponíveis.

Para começar, use um processo de assistente por VM. No Python, mantenha `RAG_WORKERS` pelo menos igual à concorrência máxima e registre-o. Não iguale contagens de threads entre virtual threads, asyncio e goroutines: o custo desses modelos de execução faz parte da comparação.

Inicie o **MCP nativo da linguagem na mesma VM**, no loopback, antes do assistente. As três implementações têm seu próprio servidor; Python e Go não dependem de um servidor Java. Os clientes MCP são inicializados na subida dos assistentes e fazem verificações periódicas, mesmo quando a rota escolhida usa ferramentas locais.

A rota `/api/v1/workflow` usa ferramentas locais. O servidor MCP deve permanecer disponível, mas seu PID não entra nos números de CPU e memória do assistente. A atividade do cliente MCP dentro do assistente faz parte desse processo. Como os dois processos compartilham a VM, a coleta por PID não elimina uma possível disputa por recursos.

Suba cada aplicação pelo seu README: [Java](../java/README.md), [Python](../python/README.md), [Go](../go/README.md). Os READMEs explicam os caminhos dos dados e o bind para acesso pela rede.

Como alternativa para estudar a integração MCP, os três clientes podem apontar para um mesmo servidor MCP Java. Registre a máquina e a rede desse servidor e mantenha essa topologia em todas as execuções desse cenário. Esse arranjo é opcional e não é o cenário principal deste guia.

## 2. Escolher uma rota e uma entrada por execução

| Rota | Cenário | Mensagem inicial sugerida |
|---|---|---|
| `/api/v1/workflow` | Principal: classificação, roteamento e ferramentas locais | `Qual o status do pedido TR-1001?` |
| `/api/v1/supervisor` | Separado: escolha de agentes pelo supervisor | `Qual o status do pedido TR-1001?` |
| `/api/v1/direct` | Separado: especialista de trocas | `Quero trocar o item sem uso do pedido TR-1002. Qual o prazo?` |
| `/api/v1/mcp` | Separado: ferramenta remota via MCP | `Qual o status do pedido TR-1001?` |

Use os mesmos textos em Java, Python e Go. `summary` vazio representa uma conversa nova; um caso com histórico constitui outra condição. Para ampliar a avaliação, repita workflow e supervisor com o caso de troca e MCP com TR-1002, identificando cada caso separadamente.

Na rota `/api/v1/mcp`, o tempo do JMeter inclui a comunicação e a execução no servidor MCP. Coletar apenas o PID do assistente deixa o consumo do servidor fora da medida, mas não retira seu efeito da latência. Esse ensaio avalia a integração entre cliente e servidor e deve ter resultados separados dos de `/workflow`.

Faça algumas chamadas manuais e confira a resposta antes da carga. Colete `/api/v1/metrics` antes e depois, com tráfego parado. Quantidade de completions, tools e tokens precisa acompanhar os números de desempenho. Mesmo com temperatura zero, o modelo pode delegar de maneira diferente ou produzir respostas de tamanhos distintos; menos etapas podem explicar uma resposta mais rápida.

## 3. Usar o JMX único

Abra [jmeter/agentic.jmx](../jmeter/agentic.jmx) no JMeter 5.6.3. O plano é autossuficiente: não depende de CSV, properties ou scripts externos. Cada cliente envia **uma solicitação por iteração**.

Na GUI, selecione **Variáveis do cenário** e substitua o valor inteiro das variáveis. Os valores iniciais usam `__P` para também aceitar parâmetros `-J` no CLI.

| Campo / propriedade CLI | Inicial | Uso |
|---|---|---|
| `host` | `127.0.0.1` | IP da VM quando o gerador for externo |
| `port` | `8080` | Java 8080, Python 8082 ou Go 8084 |
| `protocol` / `path` | `http` / `/api/v1/workflow` | Uma rota por execução |
| `message` / `summary` | Consulta TR-1001 / vazio | Mesma entrada nas três linguagens |
| `expectedRegex` | Vazio | Expressão opcional para conteúdo de `answer` |
| `threads` / `ramp` / `loops` | `1` / `1` / `1` | Clientes, rampa em segundos e iterações por cliente |
| `connectTimeout` / `timeout` | `5000` / `120000` | Timeouts em milissegundos |

Ajuste threads, ramp e loops no **Clientes** ou pelo CLI. Se substituir uma expressão `__P` por um valor literal e salvar o plano, esse campo deixa de obedecer ao `-J`; mantenha as expressões no plano usado pelo CLI. Para conferir funcionalmente, use um cliente e poucas iterações. O listener **Resumo** mostra as amostras. O plano exige HTTP 200, `answer` não vazio e `summary` string. A expressão opcional acrescenta uma checagem de conteúdo, por exemplo `(?isu).*TR-1001.*`; ela não substitui avaliar se a resposta está correta.

Para medir carga, use o CLI, conforme o [manual do JMeter](https://jmeter.apache.org/usermanual/get-started.html). Copie a pasta `jmeter` para a máquina geradora e, no diretório que a contém:

```bash
mkdir -p resultados
jmeter -n -t jmeter/agentic.jmx \
  -Jhost=IP_DA_VM -Jport=8080 \
  -Jpath=/api/v1/workflow \
  '-Jmessage=Qual o status do pedido TR-1001?' \
  -Jthreads=4 -Jramp=4 -Jloops=100 \
  -l resultados/java-workflow-pedido-01.jtl \
  -j resultados/java-workflow-pedido-01.log \
  -e -o resultados/java-workflow-pedido-01-html
```

No Windows, use `jmeter.bat`; no PowerShell, coloque o comando numa linha ou use o continuador próprio do shell. Preserve os padrões CSV do JMeter 5.6.3: cabeçalho e timestamp em milissegundos. Cada execução precisa de JTL e diretório HTML novos, para não acrescentar amostras a arquivos anteriores.

Troque a porta e os nomes de saída para Python e Go, mantendo os outros parâmetros. O plano usa carga fechada: cada cliente aguarda a resposta antes da próxima solicitação. Mesma concorrência não significa mesma taxa de chegada; a vazão resulta do atendimento.

## 4. Aquecer e repetir

Faça um aquecimento antes de medir e descarte esse JTL da análise. Use o mesmo critério nas três versões, observando a estabilização inicial da latência e CPU. Registre duração e quantidade de solicitações. Tempo de inicialização deve ser outro ensaio, caso seja de interesse.

Como matriz inicial proposta, use concorrências 1, 4, 16 e 32 e pelo menos três repetições por condição. Ajuste iterações para obter uma janela útil aos percentis, considerando custo e quotas da OCI. Registre o número efetivo de amostras.

Alterne a ordem das linguagens, por exemplo Java → Python → Go e depois Go → Java → Python. Aguarde o fim da carga e da coleta de uma VM antes de iniciar a seguinte. Mantenha a mesma configuração e o mesmo critério de aquecimento entre repetições.

## 5. Observar e registrar recursos

Use **htop para acompanhar** e **pidstat para registrar**. Instale `htop` e `sysstat` pelo gerenciador de pacotes da distribuição. Identifique o PID do assistente pela lista de processos ou pela porta de escuta, conferindo-o a cada reinicialização. Use somente esse PID na coleta principal. Se acompanhar o MCP, grave sua coleta em outro arquivo, como observação de apoio, sem somar os processos.

Substitua 12345 pelo PID real:

```bash
htop -p 12345
```

No htop, `H` alterna a exibição das threads de usuário. Não some a memória das linhas dessas threads: elas compartilham o espaço do processo. A tela ajuda a observar saturação, mas não guarda por si só a janela de medição.

Em outro terminal da VM, crie um diretório de resultados e registre CPU/memória a cada segundo:

```bash
mkdir -p resultados
pidstat -u -r -p 12345 1 > resultados/java-workflow-pedido-01-recursos.txt
```

Inicie a coleta antes da carga e interrompa com `Ctrl+C` depois dela. Anote os horários de início/fim da janela. Segundo o [manual do pidstat](https://raw.githubusercontent.com/sysstat/sysstat/master/man/pidstat.1), `-u` coleta CPU e `-r` coleta memória. Sem `-I`, 100% corresponde aproximadamente a um processador lógico ocupado; processos com várias threads podem superar 100%. RSS é informado em KiB.

Compare RSS do processo inteiro, incluindo heap, stacks, bibliotecas e memória nativa ONNX. Heap Java isolado e VSZ não são equivalentes ao RSS. O pico amostrado a cada segundo pode perder um pico intermediário; identifique-o como **máximo amostrado**. Se converter KiB em MiB, divida por 1024 e mantenha a unidade no relatório.

Opcionalmente, inicie a aplicação com GNU time; exemplo a partir de `java`:

```bash
/usr/bin/time -v -o resultados/java-ciclo-completo.txt \
  java -jar assistant/target/assistant.jar
```

Esse relatório aparece após encerrar o processo. CPU e RSS máximo abrangem todo o ciclo, incluindo inicialização e aquecimento, não somente a carga. Veja o [manual do GNU time](https://www.gnu.org/software/time/manual/html_node/Memory-Resources.html).

## 6. Interpretar resultados

Com tráfego parado, salve as métricas antes e depois de cada ensaio. Para Java:

```bash
curl -sS http://127.0.0.1:8080/api/v1/metrics > resultados/java-01-metricas-antes.json
```

Repita depois da carga, mudando o nome para `metricas-depois.json`. Subtraia os contadores cumulativos e compare chamadas, tools e tokens com as solicitações concluídas. Dados de tokens ausentes não significam consumo zero.

Use o [dashboard HTML](https://jmeter.apache.org/usermanual/generating-dashboard.html) para estatísticas e gráficos. Compare p50/p95/p99 do **tempo completo de resposta**. O campo `Latency` do JMeter descreve outra medida, relacionada ao início da resposta.

Registre, por repetição:

| Medida | O que anotar |
|---|---|
| Identificação | Linguagem/versão, rota, mensagem, clientes e repetição |
| Atendimento | Total, sucessos, erros e requisições bem-sucedidas/s |
| Tempo de resposta | p50, p95 e p99 em ms |
| CPU do assistente | Média e pico amostrados, com a convenção de percentual |
| Memória do assistente | RSS médio e máximo amostrado, na mesma unidade |
| Modelo | Completions, tools, tokens e erros reportados |

Se calcular vazão útil manualmente, a pergunta é quantos atendimentos válidos ocorreram por segundo: divida as solicitações com assertions aprovadas pela duração da mesma janela em segundos. Não confunda isso com tokens/s da OCI.

Não exclua 429, timeouts ou respostas incorretas para melhorar os números. Falhas rápidas podem reduzir os percentis sem melhorar o atendimento. Mostre a dispersão entre repetições; não calcule um percentil global pela média de percentis individuais.

Reserve as VMs ao ensaio e mantenha os serviços de fundo equivalentes. Se forem necessários limites explícitos, aplique a mesma fronteira com [systemd resource control](https://freedesktop.org/software/systemd/man/latest/systemd.resource-control.html) e registre quais processos ela abrange. `CPUQuota=200%` significa banda de duas CPUs equivalentes, não dois núcleos reservados. Memória de cgroup e RSS também são métricas distintas.

Referências consultadas em 29/10/2026. Veja também o [manual do htop](https://raw.githubusercontent.com/htop-dev/htop/main/htop.1.in).
