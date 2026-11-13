package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.declarative.Output;
import dev.langchain4j.agentic.declarative.SequenceAgent;
import dev.langchain4j.service.V;
import lab.trilha.http.Messages.Response;

@Ai.Agent("workflow")
public interface SupportWorkflow {
    @SequenceAgent(outputKey = "response", subAgents = {IntentAgent.class, RouterAgent.class, SummaryAgent.class})
    Response chat(@V("question") String question, @V("previousSummary") String previousSummary);

    @Output
    static Response response(@V("answer") String answer, @V("nextSummary") String summary) {
        return new Response(answer, summary);
    }
}
