package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.declarative.Output;
import dev.langchain4j.agentic.declarative.SequenceAgent;
import dev.langchain4j.service.V;
import lab.trilha.http.Messages.Response;

/** Mantém consulta MCP e resumo no mesmo escopo da requisição. */
@Ai.Agent("mcp-workflow")
public interface McpWorkflow {
    @SequenceAgent(outputKey = "response", subAgents = {McpOrderAgent.class, SummaryAgent.class})
    Response chat(@V("question") String question, @V("previousSummary") String previousSummary);

    @Output
    static Response response(@V("answer") String answer, @V("nextSummary") String summary) {
        return new Response(answer, summary);
    }
}
