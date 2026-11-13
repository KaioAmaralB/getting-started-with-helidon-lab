# Atribuições de terceiros

O domínio fictício Trilha e a organização didática do laboratório são apresentados no README. Esta página identifica referências e trechos de terceiros utilizados pelo projeto.

## LangChain4j

O arquivo [`comparison/contract.json`](comparison/contract.json) contém templates de planejamento e formatação oriundos do LangChain4j, preservados para que Python e Go utilizem os mesmos contratos da implementação Java. Isso inclui os prompts do planejador do supervisor e os complementos de formato de respostas e de RAG.

- Projeto de origem: [LangChain4j 1.18.1](https://github.com/langchain4j/langchain4j/tree/1.18.1).
- Versão dos módulos agênticos de referência: `1.18.1-beta28`.
- Licença dos trechos de origem: [Apache License 2.0](LICENSES/Apache-2.0.txt), copiada do [arquivo LICENSE dessa versão](https://github.com/langchain4j/langchain4j/blob/1.18.1/LICENSE).

Os prompts de atendimento da loja e suas políticas estão separados dos templates internos do framework no contrato compartilhado.

## Helidon

O [laboratório oficial langchain4j-agentic do Helidon](https://github.com/helidon-io/helidon-labs/tree/3d8f16caaa16958debe07cef681c819c87f83d5d/hols/langchain4j-agentic) é a referência técnica dos padrões declarativos explorados neste projeto. O caso de uso deste repositório é o atendimento de pedidos e trocas da loja fictícia Trilha.

## Dependências e assets

As bibliotecas Java, Python e Go são obtidas pelos gerenciadores de dependências e conservam suas próprias licenças. SDKs, ambientes virtuais, executáveis e bibliotecas nativas locais ficam fora do conteúdo versionado.

Os arquivos ONNX e o tokenizer são extraídos das dependências de versões fixadas, conforme [`comparison/assets/README.md`](comparison/assets/README.md). Sua origem e seus hashes constam em [`manifest.json`](comparison/assets/manifest.json). Esses binários extraídos também ficam fora do Git.

A cópia de Apache License 2.0 acompanha os trechos de origem identificados acima; este aviso não atribui uma licença global ao restante do código autoral.
