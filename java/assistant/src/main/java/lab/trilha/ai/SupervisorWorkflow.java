package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.declarative.Output;
import dev.langchain4j.agentic.declarative.SequenceAgent;
import dev.langchain4j.service.V;
import lab.trilha.http.Messages.Response;

/** Compõe o planejamento dinâmico com a etapa fixa de resumo. */
@Ai.Agent("supervisor-workflow")
public interface SupervisorWorkflow {
    @SequenceAgent(outputKey = "response", subAgents = {SupportSupervisor.class, SummaryAgent.class})
    Response chat(@V("question") String question, @V("previousSummary") String previousSummary);

    @Output
    static Response response(@V("answer") String answer, @V("nextSummary") String summary) {
        return new Response(answer, summary);
    }
}
