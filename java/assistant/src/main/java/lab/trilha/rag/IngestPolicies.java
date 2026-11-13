package lab.trilha.rag;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import dev.langchain4j.data.document.Metadata;
import dev.langchain4j.data.segment.TextSegment;
import dev.langchain4j.model.embedding.onnx.allminilml6v2q.AllMiniLmL6V2QuantizedEmbeddingModel;
import dev.langchain4j.store.embedding.inmemory.InMemoryEmbeddingStore;

/** Gera um segmento por arquivo Markdown usando embeddings locais. */
public final class IngestPolicies {
    private IngestPolicies() { }
    public static void main(String[] args) throws Exception {
        Path directory = Path.of(args.length == 0 ? "knowledge" : args[0]);
        var model = new AllMiniLmL6V2QuantizedEmbeddingModel();
        for (String topic : new String[]{"orders", "returns"}) {
            String content = Files.readString(directory.resolve(topic + ".md"), StandardCharsets.UTF_8);
            var segment = TextSegment.from(content, Metadata.from("source", topic + ".md"));
            var store = new InMemoryEmbeddingStore<TextSegment>();
            store.add(topic + "-v1", model.embed(segment).content(), segment);
            Path output = directory.resolve(topic + "-embeddings.json");
            store.serializeToFile(output);
            System.out.println("Gerado: " + output + " (1 chunk)");
        }
    }
}
