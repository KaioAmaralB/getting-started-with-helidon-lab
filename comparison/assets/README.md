# Modelo, tokenizer e biblioteca ONNX

O modelo, o tokenizer e as bibliotecas nativas são usados no RAG dos assistentes Python e Go; os servidores MCP não os carregam. Esses binários não são versionados. Prepare-os manualmente depois de compilar Java. Todos os comandos abaixo partem da raiz do repositório.

O arquivo `manifest.json` identifica a origem e os hashes da preparação atual. O modelo/tokenizer vêm de `langchain4j-embeddings-all-minilm-l6-v2-q:1.18.1-beta28`; a biblioteca nativa vem de `onnxruntime:1.20.0`. Não substitua versões somente em uma linguagem.

## Linux x86-64

Com `unzip` instalado:

```bash
mkdir -p comparison/assets comparison/.tools/onnxruntime
unzip -p java/assistant/target/libs/langchain4j-embeddings-all-minilm-l6-v2-q-1.18.1-beta28.jar \
  all-minilm-l6-v2-q.onnx > comparison/assets/model.onnx
unzip -p java/assistant/target/libs/langchain4j-embeddings-all-minilm-l6-v2-q-1.18.1-beta28.jar \
  all-minilm-l6-v2-q-tokenizer.json > comparison/assets/tokenizer.json
unzip -p java/assistant/target/libs/onnxruntime-1.20.0.jar \
  ai/onnxruntime/native/linux-x64/libonnxruntime.so \
  > comparison/.tools/onnxruntime/libonnxruntime.so
```

Esses comandos extraem bytes dos mesmos JARs usados pelo Java; não fazem inferência nem treinamento. Confira o código de saída do `unzip` antes de usar um arquivo gerado. Os exemplos são x86-64; para outra arquitetura, escolha a biblioteca correspondente no JAR e use o mesmo destino configurado no Go.

## Windows x64

Abra os JARs como arquivos ZIP no seu gerenciador de arquivos:

1. No JAR `langchain4j-embeddings-all-minilm-l6-v2-q-1.18.1-beta28.jar`, extraia `all-minilm-l6-v2-q.onnx` e `all-minilm-l6-v2-q-tokenizer.json`.
2. Coloque-os nesta pasta com nomes `model.onnx` e `tokenizer.json`.
3. No JAR `onnxruntime-1.20.0.jar`, extraia `ai/onnxruntime/native/win-x64/onnxruntime.dll` para `../.tools/onnxruntime/onnxruntime.dll`.

Os JARs ficam em `java/assistant/target/libs` a partir da raiz do repositório. A extração não precisa alterar os originais. O Go deve usar a DLL; o pacote Python instala sua própria distribuição ONNX Runtime 1.20.0.

## Índices e correspondência

A ingestão Java gera `java/knowledge/orders-embeddings.json` e `returns-embeddings.json`. Python e Go leem esses arquivos como dados, sem executar Java durante as requisições. No assistente Go, `LAB_ROOT` aponta para `java` por causa desse diretório `knowledge`; contrato e assets têm caminhos separados, conforme seu README. Gere-os novamente somente quando mudar as políticas; use a mesma versão nas três aplicações.

O modelo é baseado no [all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2). O comportamento preservado nesta implementação usa truncamento de 128 tokens, CPU e normalização float32. O corpus didático tem um trecho por domínio; isso não demonstra qualidade de ranking em um corpus maior.
