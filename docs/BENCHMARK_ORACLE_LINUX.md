# Benchmark manual em três VMs Oracle Linux

Este roteiro prepara **uma VM para Java, uma para Python e uma para Go**, com instalação dos runtimes, envio dos arquivos, configuração OCI e coleta manual. O cenário principal usa `POST /api/v1/workflow` e mede CPU e memória do processo do assistente em cada VM.

Os blocos **computador local** são executados no Git Bash do Windows. Os blocos **VM** são executados depois do SSH, no Oracle Linux. A sintaxe Bash parecida não torna seus binários intercambiáveis: a `.venv` Windows e os `.exe` não serão enviados.

## 1. Criar as instâncias e definir a rede

Use a **mesma imagem Oracle Linux 9.8 x86-64**, shape, OCPUs, RAM, boot volume e configuração de rede nas três VMs. Essa é a referência do documento; não misture Arm e x86.

Uma configuração inicial proposta é **2 OCPUs e 8 GB de RAM por VM**, numa shape x86 Flex disponível na região. Isso não é capacidade medida do laboratório. Em OCI x86, duas OCPUs correspondem a quatro vCPUs; registre as duas unidades. Confira quota e custos antes de provisionar.

Na Console OCI, em **Compute → Instances → Create instance**, crie:

| Instância | Instalação principal | Assistente | MCP local |
|---|---|---:|---:|
| `trilha-java` | JDK 21 e Maven 3.9 | 8080 | Java, 8081 |
| `trilha-python` | CPython 3.13 | 8082 | Python, 8083 |
| `trilha-go` | Go 1.27.3 e GCC | 8084 | Go, 8085 |

Selecione a mesma imagem e recursos em cada criação, associe sua chave pública SSH e anote os IPs. As imagens Oracle Linux usam o usuário `opc`.

O **JMeter fica fora das três VMs**. Seu computador pode ser o gerador; uma quarta VM de carga é opcional. Não execute o gerador somente na VM de uma das linguagens.

### Acesso e portas

Os exemplos de SSH/SCP pressupõem IPs públicos acessíveis e uma subnet com rota pelo Internet Gateway. Configure NSGs/regras de segurança:

- TCP 22: somente a origem administrativa.
- TCP 8080 na Java, 8082 na Python e 8084 na Go: somente o IP/CIDR do gerador.
- As portas MCP ficam no loopback, sem ingresso externo.
- Mantenha DNS e saída HTTPS para os repositórios e o endpoint regional GenAI.

O firewall da VM também deve permitir o tráfego. Em uma subnet privada, use um gerador com acesso à VCN e SSH por VPN, Bastion ou outra rota administrativa. NAT fornece saída para downloads, não SSH de entrada; Service Gateway não substitui o acesso a Go, PyPI e demais repositórios públicos.

Use o mesmo tipo de caminho nas três medições. Se o gerador acessar IPs públicos pela internet, registre essa condição e sua variabilidade.

### Por que um MCP por VM?

Cada VM terá **dois processos da própria linguagem**: assistente e MCP. As três implementações já incluem seus próprios servidores. Python e Go não precisam de Java para executá-los. Os clientes MCP são inicializados na subida dos assistentes e fazem verificações periódicas, por isso o servidor deve estar disponível mesmo para testar uma rota que usa ferramentas locais.

No cenário principal, `/api/v1/workflow` usa ferramentas locais. **Somente o PID do assistente entra na coleta de CPU e memória**: HTTP, orquestração, RAG, bibliotecas nativas, cliente OCI e a atividade do cliente MCP nesse processo. O servidor MCP fica fora dessa medida. Ele compartilha recursos da VM com o assistente, portanto isso não equivale a isolar fisicamente os dois processos.

A rota `/api/v1/mcp` é um ensaio separado. Seu tempo de resposta inclui comunicação e execução no servidor MCP, mesmo quando os recursos são coletados apenas no PID do assistente. Para estudar essa integração com um servidor fixo, é possível apontar os três clientes para um MCP Java comum. Registre essa topologia e seus resultados à parte; os comandos deste roteiro usam o MCP nativo em cada VM.

## 2. Empacotar no Windows

No **Git Bash local**, acesse a raiz do repositório, onde estão `java/`, `python/`, `go/`, `comparison/` e `jmeter/`. O nome da pasta de clone é livre. Confira os arquivos necessários:

```bash
ls java/pom.xml python/requirements.lock go/go.mod
ls comparison/assets/model.onnx comparison/assets/tokenizer.json
ls java/knowledge/orders-embeddings.json java/knowledge/returns-embeddings.json
ls java/assistant/target/libs/onnxruntime-1.20.0.jar
```

Se faltar algum dado, prepare-o conforme [Java](../java/README.md) e [assets](../comparison/assets/README.md) antes de empacotar. Use os mesmos modelos, índices e políticas nas três VMs. O JAR ONNX é uma dependência pública do build Java; a VM Go o abrirá como ZIP para extrair a biblioteca Linux, sem executar Java.

Os comandos abaixo usam **GNU tar** (`tar --version`) e acrescentam o prefixo `labs/` aos caminhos do pacote. Assim, os arquivos serão extraídos em `~/trilha/labs` nas VMs, independentemente do nome da pasta clonada no computador local. O prefixo faz parte do formato dos pacotes deste roteiro.

Mantenha chaves SSH e segredos fora da pasta do repositório. Os YAMLs usados no pacote devem manter referências a variáveis de ambiente; informe a API key somente no terminal da VM. Pacotes de transferência são locais e ficam fora do Git.

**Pacote Java:**

```bash
tar -czf "$HOME/trilha-java.tar.gz" \
  --transform='s,^,labs/,' \
  --exclude='*/target' --exclude='*/.git' \
  --exclude='*.log' --exclude='*/.env*' \
  --exclude='*.pem' --exclude='*.key' \
  java
```

**Pacote Python:**

```bash
tar -czf "$HOME/trilha-python.tar.gz" \
  --transform='s,^,labs/,' \
  --exclude='*/.venv' --exclude='*/.cache' \
  --exclude='*/__pycache__' --exclude='*.pyc' \
  --exclude='*.log' --exclude='*/.env*' \
  --exclude='*.pem' --exclude='*.key' \
  python \
  java/knowledge \
  comparison/contract.json \
  comparison/assets/model.onnx \
  comparison/assets/tokenizer.json
```

**Pacote Go:**

```bash
tar -czf "$HOME/trilha-go.tar.gz" \
  --transform='s,^,labs/,' \
  --exclude='*/bin' --exclude='*/.cache' \
  --exclude='*.log' --exclude='*/.env*' \
  --exclude='*.pem' --exclude='*.key' \
  go \
  java/knowledge \
  comparison/contract.json \
  comparison/assets/model.onnx \
  comparison/assets/tokenizer.json \
  java/assistant/target/libs/onnxruntime-1.20.0.jar
```

O Go recebe somente aquele JAR de dependência, sem os executáveis Windows. `java/knowledge` também precisa existir nas VMs Python e Go: é o diretório dos dados, não uma exigência de JDK.

Confira a lista dos pacotes e seus hashes:

```bash
tar -tzf "$HOME/trilha-java.tar.gz"
tar -tzf "$HOME/trilha-python.tar.gz"
tar -tzf "$HOME/trilha-go.tar.gz"
sha256sum "$HOME/trilha-java.tar.gz" "$HOME/trilha-python.tar.gz" "$HOME/trilha-go.tar.gz"
```

A lista deve começar com `labs/` e não deve conter ambientes virtuais, caches, chaves, segredos ou executáveis Windows. Guarde os hashes para conferir a transferência.

## 3. Enviar com SCP e conectar por SSH

No **Git Bash local**, substitua os valores:

```bash
CHAVE_SSH="$HOME/.ssh/oci-benchmark.key"
IP_JAVA='<IP de administração da VM Java>'
IP_PYTHON='<IP de administração da VM Python>'
IP_GO='<IP de administração da VM Go>'
chmod 400 "$CHAVE_SSH"
```

A chave SSH é o arquivo de acesso à VM; ela é diferente da API key GenAI, que será usada pela aplicação.

```bash
scp -i "$CHAVE_SSH" "$HOME/trilha-java.tar.gz" "opc@$IP_JAVA:/home/opc/"
scp -i "$CHAVE_SSH" "$HOME/trilha-python.tar.gz" "opc@$IP_PYTHON:/home/opc/"
scp -i "$CHAVE_SSH" "$HOME/trilha-go.tar.gz" "opc@$IP_GO:/home/opc/"
```

Acesse cada host em uma aba própria do **Git Bash local**. Defina `CHAVE_SSH` e o IP correspondente em cada aba, ou substitua-os pelos valores reais nos comandos. Execute apenas o comando correspondente; não cole os três numa sessão já conectada.

**Aba Java:**

```bash
ssh -i "$CHAVE_SSH" "opc@$IP_JAVA"
```

**Aba Python:**

```bash
ssh -i "$CHAVE_SSH" "opc@$IP_PYTHON"
```

**Aba Go:**

```bash
ssh -i "$CHAVE_SSH" "opc@$IP_GO"
```

Na primeira conexão, confira a identidade da instância antes de aceitar sua chave de host.

## 4. Preparar o sistema nas três VMs

A partir daqui, execute **dentro da VM**, como `opc`. Confira e registre:

```bash
cat /etc/oracle-release
uname -m
uname -r
lscpu
free -h
```

O roteiro espera Oracle Linux 9 e `x86_64`. Mesma shape não garante o mesmo host físico; registre o modelo de CPU e a variação entre repetições.

Nas três VMs:

```bash
sudo dnf update -y
sudo dnf install -y dnf-plugins-core oracle-epel-release-el9
sudo dnf config-manager --enable ol9_developer_EPEL
sudo dnf install -y tar gzip unzip openssh-clients sysstat htop tmux file
```

Developer EPEL fornece ferramentas de desenvolvimento e também o Python 3.13 deste roteiro. Seu suporte tem escopo distinto dos pacotes centrais Oracle Linux; a escolha é para o laboratório.

Confira `curl --version`. Se não existir, instale `curl` pelo DNF. Se houver reinicialização pendente após atualizar, reinicie as três VMs antes da carga e só depois inicie as aplicações. Registre novamente a versão do sistema e do kernel após a atualização/reboot.

### Extrair o pacote correspondente

Na **VM Java**:

```bash
cd ~
sha256sum trilha-java.tar.gz
mkdir -p ~/trilha
tar -xzf trilha-java.tar.gz -C ~/trilha
```

Na **VM Python**:

```bash
cd ~
sha256sum trilha-python.tar.gz
mkdir -p ~/trilha
tar -xzf trilha-python.tar.gz -C ~/trilha
```

Na **VM Go**:

```bash
cd ~
sha256sum trilha-go.tar.gz
mkdir -p ~/trilha
tar -xzf trilha-go.tar.gz -C ~/trilha
```

Compare os hashes com os do Windows. O destino uniforme é `/home/opc/trilha/labs`. Use diretórios limpos para uma nova implantação, evitando misturar versões.

### Abrir a porta do assistente no firewall

Em cada VM:

```bash
sudo firewall-cmd --get-active-zones
```

Exemplo para a **VM Java**, trocando a origem pelo CIDR real do gerador e `public` pela zona encontrada:

```bash
sudo firewall-cmd --permanent --zone=public \
  --add-rich-rule='rule family="ipv4" source address="10.0.0.50/32" port port="8080" protocol="tcp" accept'
sudo firewall-cmd --reload
```

Nas VMs Python e Go, use 8082 e 8084. `10.0.0.50/32` é ilustrativo: substitua pela origem que efetivamente chega à VM, pública ou privada. NSG e firewall precisam concordar. Não abra as portas MCP.

## 5. Java: instalar e compilar na VM Java

```bash
sudo dnf install -y java-21-openjdk-devel
sudo dnf module install -y maven:3.9
sudo dnf install -y maven-openjdk21
java -version
javac -version
mvn -version
```

O stream Maven 3.9 está documentado a partir do OL9.6. Confirme Maven 3.9.9 ou superior e JDK 21. Se houver mais de um Java, selecione o 21:

```bash
sudo alternatives --config java
sudo alternatives --config javac
export JAVA_HOME="$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")"
export PATH="$JAVA_HOME/bin:$PATH"
mvn -version
```

Compile:

```bash
cd ~/trilha/labs/java
mvn -B -DskipTests package
```

Preserve `assistant/target/libs` e `mcp-server/target/libs` junto dos JARs. Reutilize os índices de `knowledge` enviados; não rode nova ingestão só nesta VM durante a preparação do benchmark.

## 6. Python: instalar na VM Python

O Python padrão do sistema não é necessariamente o 3.13. Consulte e instale a versão nomeada:

```bash
dnf info python3.13 python3.13-pip
sudo dnf install -y python3.13 python3.13-pip
python3.13 --version
rpm -q python3.13 python3.13-libs python3.13-pip
```

A série 3.13 é fornecida pelo Developer EPEL neste roteiro, não deve ser presumida como padrão/AppStream. O patch DNF pode diferir do 3.13.14 usado no Windows; registre o valor real. Se `dnf info` não encontrar os pacotes, confira `dnf repolist --enabled` e o canal da imagem, sem substituir silenciosamente por 3.9 ou 3.14.

Crie o ambiente **na VM**:

```bash
cd ~/trilha/labs/python
python3.13 -m venv .venv
.venv/bin/python -m pip install --only-binary=:all: -r requirements.lock
.venv/bin/python -m pip check
.venv/bin/python --version
```

O lock mantém NumPy 2.1.3, ONNX Runtime 1.20.0 e tokenizers 0.20.3. Há wheels Linux x86-64 para CPython 3.13. Use o Python convencional, sem build debug/free-threaded ou JIT experimental neste cenário. Não substitua o Python do sistema.

## 7. Go: instalar e compilar na VM Go

```bash
sudo dnf install -y gcc gcc-c++ glibc-devel libstdc++ make
mkdir -p ~/downloads
cd ~/downloads
curl -fLO https://go.dev/dl/go1.27.3.linux-amd64.tar.gz
printf '%s  %s\n' \
  '65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40' \
  'go1.27.3.linux-amd64.tar.gz' | sha256sum -c -
```

O hash é o publicado no catálogo Go. Prossiga somente se indicar `OK`. Instale num diretório novo:

```bash
sudo mkdir /opt/go-1.27.3
sudo tar -xzf go1.27.3.linux-amd64.tar.gz \
  -C /opt/go-1.27.3 --strip-components=1
export PATH="/opt/go-1.27.3/bin:$PATH"
export CGO_ENABLED=1
export CC=gcc
go version
gcc --version
```

Se o destino já existir, confira antes de reutilizá-lo; não extraia sobre outro SDK. Esse PATH vale para a sessão e precisa ser configurado em outra sessão de build.

### Preparar ONNX Linux sem instalar Java

Extraia a biblioteca do mesmo JAR público usado pelo Java:

```bash
cd ~/trilha/labs
mkdir -p comparison/.tools/onnxruntime
unzip -p java/assistant/target/libs/onnxruntime-1.20.0.jar \
  ai/onnxruntime/native/linux-x64/libonnxruntime.so \
  > comparison/.tools/onnxruntime/libonnxruntime.so
file comparison/.tools/onnxruntime/libonnxruntime.so
ldd comparison/.tools/onnxruntime/libonnxruntime.so
```

Confirme sucesso do `unzip`, ELF x86-64 e ausência de `not found` no `ldd`. Não use a DLL Windows. Python instala sua própria distribuição ONNX pelo lock; modelo e versão do runtime permanecem alinhados.

Compile os executáveis Linux:

```bash
cd ~/trilha/labs/go
go mod download
go build -o bin/assistant ./cmd/assistant
go build -o bin/mcp-server ./cmd/mcp-server
```

O Go usa CGO para carregar ONNX no próprio processo. Os executáveis Linux não precisam de `.exe`.

## 8. Manter as sessões e configurar OCI

### Criar as janelas antes de informar o segredo

Em cada VM, use duas janelas. Primeiro execute `tmux new -s trilha`. Na primeira janela, inicie o MCP conforme a seção 9. Pressione **Ctrl+B, depois C** para criar a janela do assistente. É nessa nova janela que você deve informar a chave OCI e iniciar o assistente.

As variáveis exportadas em outro terminal ou numa janela já aberta não são compartilhadas automaticamente. Configure-as na própria janela do assistente.

**Ctrl+B, depois D** desconecta mantendo os processos. `tmux attach -t trilha` retoma a sessão. O tmux não reinicia aplicações após um reboot; nesse caso, repita a inicialização.

Em **cada VM**, execute este bloco no terminal/janela onde iniciará o **assistente**:

```bash
export OCI_BASE_URL='https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/20231130/actions/v1'
read -rsp 'Segredo da API key do Generative AI: ' OCI_API_KEY
echo
export OCI_API_KEY
export OCI_ROUTING_MODEL='<modelo de classificação/resumo>'
export OCI_EXPERT_MODEL='<modelo dos especialistas/supervisor>'
```

Use os mesmos modelos reais nas três VMs. O exemplo é Chicago; chave e modelos devem corresponder à região escolhida e a política IAM deve autorizar a operação.

A autenticação continua sendo **API key GenAI** com Bearer token. Rodar numa VM OCI não ativa Instance Principal automaticamente. O código não lê `[DEFAULT]`/PEM nem usa `OCI_COMPARTMENT_ID`. Veja [autenticação](../README.md#autenticação-na-oci) e [permissões oficiais](https://docs.oracle.com/en-us/iaas/Content/generative-ai/add-api-permission.htm).

O MCP não precisa desse segredo. Não coloque a API key no JMeter, nos pacotes ou nos resultados.

## 9. Subir as aplicações

### VM Java

**Janela MCP:**

```bash
cd ~/trilha/labs/java
java -Dserver.host=127.0.0.1 -Dserver.port=8081 \
  -jar mcp-server/target/mcp-server.jar
```

**Janela assistente:** configure antes as variáveis OCI.

```bash
cd ~/trilha/labs/java
java -Dserver.host=0.0.0.0 -Dserver.port=8080 \
  -Dlangchain4j.mcp-clients.order-server.uri=http://127.0.0.1:8081/orders \
  -jar assistant/target/assistant.jar
```

### VM Python

**Janela MCP:**

```bash
cd ~/trilha/labs/python
LAB_ROOT="$HOME/trilha/labs" HOST=127.0.0.1 MCP_SERVER_PORT=8083 \
  .venv/bin/python -m trilha.mcp_server
```

**Janela assistente:** configure antes as variáveis OCI.

```bash
cd ~/trilha/labs/python
export LAB_ROOT="$HOME/trilha/labs"
export HOST=0.0.0.0
export PORT=8082
export MCP_URL='http://127.0.0.1:8083/orders'
export RAG_WORKERS=256
.venv/bin/python -m trilha.assistant
```

No assistente Python, `LAB_ROOT` é a raiz do repositório; os documentos e índices ficam em `LAB_ROOT/java/knowledge`. O MCP Python usa essa mesma raiz para localizar `comparison/contract.json` e não faz RAG. Esses caminhos não exigem JDK nesta VM.

### VM Go

**Janela MCP:**

```bash
cd ~/trilha/labs/go
CONTRACT_PATH="$HOME/trilha/labs/comparison/contract.json" \
HOST=127.0.0.1 MCP_SERVER_PORT=8085 ./bin/mcp-server
```

O MCP Go lê `comparison/contract.json` pelo caminho explícito. Ele não usa `java/knowledge` nem executa RAG; por isso esse comando não precisa de `LAB_ROOT`.

**Janela assistente:** configure antes as variáveis OCI.

```bash
cd ~/trilha/labs/go
export LAB_ROOT="$HOME/trilha/labs/java"
export CONTRACT_PATH="$HOME/trilha/labs/comparison/contract.json"
export EMBEDDING_ASSETS_DIR="$HOME/trilha/labs/comparison/assets"
export ONNXRUNTIME_SHARED_LIBRARY_PATH="$HOME/trilha/labs/comparison/.tools/onnxruntime/libonnxruntime.so"
export HOST=0.0.0.0
export PORT=8084
export MCP_URL='http://127.0.0.1:8085/orders'
./bin/assistant
```

No assistente Go, `LAB_ROOT` aponta para `labs/java` porque o código procura os documentos e índices em `LAB_ROOT/knowledge`. A base em `java/knowledge` é compartilhada pelas três implementações; esse caminho não exige executar Java. `CONTRACT_PATH` e `EMBEDDING_ASSETS_DIR` apontam separadamente para o contrato e o modelo em `labs/comparison`. Não é preciso instalar JDK na VM Go.

## 10. Conferir antes de medir

Em outra sessão SSH, consulte a porta correspondente. Exemplo Java:

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/api/v1/runtime
curl -sS http://127.0.0.1:8080/api/v1/workflow \
  -H 'Content-Type: application/json' \
  -d '{"message":"Qual o status do pedido TR-1001?","summary":""}'
```

Use 8082/8084 nas demais. `/health` verifica o processo; a chamada de negócio confere também modelo, RAG e tools, e consome OCI. Confira seu conteúdo antes de aumentar carga.

Compare os hashes dos índices nas três VMs:

```bash
cd ~/trilha/labs
sha256sum java/knowledge/orders-embeddings.json java/knowledge/returns-embeddings.json
```

Nas VMs Python/Go, confira também:

```bash
sha256sum comparison/contract.json comparison/assets/model.onnx comparison/assets/tokenizer.json
```

Java carrega modelo/tokenizer dos JARs. Preserve as versões do build; não atualize dependências ou dados entre repetições.

Do gerador, confira `http://IP_DA_VM:PORTA/health` de cada host. Se funcionar localmente e não remotamente, revise bind, endereço, rota, NSG e firewall.

## 11. Executar JMeter e coletar manualmente

Use o [JMX existente](../jmeter/agentic.jmx), JMeter 5.6.3 e o mesmo gerador. O plano não precisa ir às VMs de aplicação.

Aqueça cada aplicação, com o mesmo critério de estabilização, descartando os resultados do aquecimento. Meça uma linguagem por vez e alterne a ordem entre repetições. Cargas simultâneas podem disputar quotas OCI e recursos do gerador.

No **Git Bash local**, a partir da raiz do repositório, exemplo Java:

```bash
mkdir -p resultados
jmeter -n -t jmeter/agentic.jmx \
  -Jhost='<IP da VM Java acessível ao gerador>' -Jport=8080 \
  -Jpath=/api/v1/workflow \
  '-Jmessage=Qual o status do pedido TR-1001?' \
  -Jthreads=4 -Jramp=4 -Jloops=100 \
  -l resultados/java-workflow-c4-r1.jtl \
  -j resultados/java-workflow-c4-r1.log \
  -e -o resultados/java-workflow-c4-r1-html
```

Use `jmeter.bat` se esse for o nome local. Repita com IP/porta 8082 da Python e 8084 da Go, mantendo os demais parâmetros e novos nomes de saída. O plano é carga fechada: cada cliente aguarda a resposta antes da próxima solicitação.

Esses valores são uma condição inicial proposta. Faça pelo menos três repetições, registre amostras e aumente a concorrência de maneira igual. Preserve `/workflow` e a consulta TR-1001 como cenário principal. Se avaliar `/supervisor`, `/direct` ou `/mcp`, faça execuções separadas, com entradas iguais entre linguagens; para `/direct`, use uma pergunta de troca. O [guia de método](COMPARACAO_MANUAL.md) detalha os casos e percentis.

### CPU e memória na VM sob carga

Identifique o PID do assistente pela porta da aplicação (8080, 8082 ou 8084). Confira também qual é o processo MCP para não incluí-lo por engano:

```bash
ps -eo pid,comm,args
ss -ltnp
```

Evite publicar a lista completa de processos, que pode conter argumentos privados. Substitua 12345 pelo PID real do **assistente**:

```bash
htop -p 12345
```

Registre CPU e memória desse processo a cada segundo em outro terminal da VM:

```bash
mkdir -p ~/resultados
pidstat -u -r -p 12345 1 > ~/resultados/java-workflow-c4-r1-recursos.txt
```

Comece antes da carga e interrompa com Ctrl+C depois, anotando os horários da janela. RSS é informado em KiB. Sem `-I`, 100% de CPU representa aproximadamente uma CPU lógica; processos multithread podem ultrapassar 100%.

Se quiser observar o MCP, use seu PID em uma segunda coleta e salve outro arquivo, identificado como apoio. Não some esses valores aos resultados do assistente; somar RSS também pode contar páginas compartilhadas duas vezes. O pico amostrado pode perder eventos entre coletas; heap Java isolado não equivale a RSS.

Salve métricas antes/depois com tráfego parado. Exemplo na VM Java:

```bash
curl -fsS http://127.0.0.1:8080/api/v1/metrics > ~/resultados/java-metricas-antes.json
```

Repita depois, mudando o nome para `java-metricas-depois.json`. Quantidade de completions, tools e tokens precisa acompanhar latência e vazão: menos trabalho solicitado pelo modelo pode explicar menor tempo.

Para buscar a coleta no **computador local**:

```bash
scp -i "$CHAVE_SSH" "opc@$IP_JAVA:/home/opc/resultados/java-workflow-c4-r1-recursos.txt" ./resultados/
```

Repita para os snapshots e outras VMs. Guarde JTL/dashboard do gerador e coleta do servidor com o mesmo identificador.

## 12. Interpretar e encerrar

Registre por repetição: imagem/shape, OCPUs/RAM, modelo de CPU, kernel, versões, rota/caso, concorrência, sucessos/erros, vazão útil, p50/p95/p99 do tempo completo de resposta, CPU e RSS do assistente e deltas das métricas de modelo. O tempo medido pelo JMeter inclui rede e espera pela OCI; não representa apenas processamento local.

São três instâncias equivalentes, não o mesmo host físico. Mostre dispersão; não obtenha percentil global pela média dos percentis individuais. Internet/OCI, quotas, decisões do supervisor e tamanho das respostas podem dominar o resultado. Não descarte 429, timeouts ou respostas incorretas para melhorar a amostra.

Ao terminar, pare os processos e avalie desligar/remover recursos sem uso, incluindo boot volumes e demais itens cobrados. Este documento não apresenta preços ou resultados medidos.

## Problemas frequentes

| Sintoma | Conferência |
|---|---|
| Python `ArgsKwargs` ou extensão nativa não carrega | Crie `.venv` Linux limpa com `python3.13` e reinstale o lock; não copie a Windows |
| Go `*fmt.wrapError` | Confira `LAB_ROOT`, contrato/assets, `libonnxruntime.so`, `ldd` e MCP local |
| Java não encontra índices | Inicie em `~/trilha/labs/java` e confira `knowledge/*-embeddings.json` |
| Assistente encerra antes de abrir porta | Inicie primeiro o MCP da linguagem e confira caminhos |
| Saúde local responde, mas o gerador não conecta | Bind, IP, NSG, firewall e rota |
| HTTP 502 no atendimento | Chave regional GenAI, modelos, IAM, quotas e status do provedor |
| DNF não encontra Python 3.13/Maven 3.9 | Imagem OL9.8, repositórios e `dnf info`; não substitua versões silenciosamente |

## Fontes e escopo

Fontes primárias consultadas em **29/10/2026**:

- [Imagem OL9.8](https://docs.oracle.com/en-us/iaas/images/oracle-linux-9x/oracle-linux-9-8-2026-08-14-0.htm), [shapes e OCPUs/vCPUs](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) e [SSH com opc](https://docs.oracle.com/en-us/iaas/oracle-linux/oci/gs-access-instance.htm).
- [Maven 3.9/OpenJDK 21 no OL9.6+](https://docs.oracle.com/en/operating-systems/oracle-linux/9/relnotes9.6/ol9-features-CompilersandDevTools.html).
- [Repositórios Oracle Linux](https://docs.oracle.com/en/operating-systems/oracle-linux/software-management/sfw-mgmt-AvailableYumRepositories.html), [habilitação EPEL](https://docs.oracle.com/en/learn/ol-vbox/) e [catálogo OL9 Developer EPEL](https://yum.oracle.com/repo/OracleLinux/OL9/developer/EPEL/x86_64/).
- [Download Go/SHA-256](https://go.dev/dl/) e [instalação Go](https://go.dev/doc/install).
- Wheels: [NumPy 2.1.3](https://pypi.org/project/numpy/2.1.3/), [ONNX 1.20.0](https://pypi.org/project/onnxruntime/1.20.0/) e [tokenizers 0.20.3](https://pypi.org/project/tokenizers/0.20.3/).
- [Binding Go/ONNX v1.16.0](https://github.com/yalue/onnxruntime_go/tree/v1.16.0).
- [Rede OCI](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securityrules.htm) e [firewalld rich rules](https://firewalld.org/documentation/man-pages/firewalld.richlanguage.html).
- [JMeter CLI](https://jmeter.apache.org/usermanual/get-started.html) e [pidstat](https://raw.githubusercontent.com/sysstat/sysstat/master/man/pidstat.1).

As configurações e cargas propostas neste roteiro não são resultados medidos. Confirme na VM a disponibilidade dos pacotes antes de instalar e registre a configuração efetivamente utilizada.

