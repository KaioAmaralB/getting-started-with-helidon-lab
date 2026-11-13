package lab.trilha.metrics;

import java.util.concurrent.atomic.LongAdder;
import io.helidon.json.binding.Json;
import io.helidon.service.registry.Service;
import dev.langchain4j.model.chat.listener.*;

/** Contadores agregados das chamadas ao modelo. */
@Service.Singleton
@Service.Named("model-metrics")
public class ModelMetrics implements ChatModelListener {
    private final LongAdder requests = new LongAdder();
    private final LongAdder responses = new LongAdder();
    private final LongAdder errors = new LongAdder();
    private final LongAdder inputTokens = new LongAdder();
    private final LongAdder outputTokens = new LongAdder();
    private final LongAdder missingUsage = new LongAdder();
    private final LongAdder toolCalls = new LongAdder();

    @Override public void onRequest(ChatModelRequestContext context) { requests.increment(); }
    @Override public void onError(ChatModelErrorContext context) { errors.increment(); }
    @Override public void onResponse(ChatModelResponseContext context) {
        responses.increment();
        var response = context.chatResponse();
        var usage = response.tokenUsage();
        if (usage == null || usage.inputTokenCount() == null || usage.outputTokenCount() == null) { missingUsage.increment(); }
        if (usage != null && usage.inputTokenCount() != null) { inputTokens.add(usage.inputTokenCount()); }
        if (usage != null && usage.outputTokenCount() != null) { outputTokens.add(usage.outputTokenCount()); }
        if (response.aiMessage().hasToolExecutionRequests()) { toolCalls.add(response.aiMessage().toolExecutionRequests().size()); }
    }

    public Snapshot snapshot() {
        return new Snapshot(requests.sum(), responses.sum(), errors.sum(), inputTokens.sum(), outputTokens.sum(), missingUsage.sum(), toolCalls.sum());
    }
    @Json.Entity
    public record Snapshot(long modelRequests, long modelResponses, long modelErrors, long inputTokens,
                           long outputTokens, long responsesWithoutCompleteUsage, long requestedToolCalls) { }
}
