# Recursos de execução compartilhados

Esta pasta reúne os prompts, schemas e assets usados por Python e Go para executar o mesmo cenário do laboratório Java.

| Arquivo | Uso |
|---|---|
| `contract.json` | Prompts, schemas de tools, limites e parâmetros extraídos da referência Java |
| `assets/model.onnx` | MiniLM quantizado executado localmente |
| `assets/tokenizer.json` | Tokenizer do mesmo artefato Java |
| `assets/manifest.json` | Origem, versão e hashes dos assets preparados |
| `.tools/onnxruntime` | Biblioteca nativa ONNX 1.20.0 utilizada pelo Go |

A preparação dos assets é [manual](assets/README.md). As políticas e seus índices ficam em `../java/knowledge` e são lidos pelas três implementações. Essa localização evita cópias divergentes e não cria uma dependência do runtime Java no atendimento Python ou Go.

No assistente Go, `LAB_ROOT` aponta para `../java`, pois o carregador procura `knowledge` sob esse caminho. `CONTRACT_PATH` e `EMBEDDING_ASSETS_DIR` apontam separadamente para esta pasta. O servidor MCP Go precisa somente de `CONTRACT_PATH`, sem `LAB_ROOT` ou assets ONNX. No Python, `LAB_ROOT` indica a raiz do repositório, usada para localizar o contrato; somente o assistente carrega RAG.

Cada linguagem implementa seu servidor MCP. O contrato compartilhado mantém nome, argumentos e resultado da ferramenta iguais; não direciona as chamadas a um servidor Java.

Ambientes, bibliotecas nativas, SDKs e compiladores locais ficam fora do Git. Instale Go e um compilador C compatível no seu ambiente; o clone não contém ferramentas portáteis.

Os blocos de planejamento oriundos do LangChain4j continuam atribuídos ao [projeto, sob Apache 2.0](https://github.com/langchain4j/langchain4j/blob/1.18.1/LICENSE). A estrutura efetiva do contrato usa as versões fixadas nas dependências Java.

Para executar, consulte os READMEs de [Java](../java/README.md), [Python](../python/README.md) e [Go](../go/README.md). A comparação é descrita em [COMPARACAO_MANUAL.md](../docs/COMPARACAO_MANUAL.md).
