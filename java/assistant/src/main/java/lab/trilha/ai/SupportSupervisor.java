package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.declarative.SupervisorAgent;
import dev.langchain4j.agentic.declarative.SupervisorRequest;
import dev.langchain4j.agentic.supervisor.SupervisorResponseStrategy;
import dev.langchain4j.service.V;

@Ai.Agent("supervisor")
@Ai.ChatModel("expert-model")
public interface SupportSupervisor {
    @SupervisorAgent(outputKey = "answer", subAgents = {OrderAgent.class, ReturnAgent.class}, maxAgentsInvocations = 3, responseStrategy = SupervisorResponseStrategy.SUMMARY)
    String chat(@V("question") String question, @V("previousSummary") String previousSummary);
    @SupervisorRequest
    static String request(@V("question") String question, @V("previousSummary") String summary) {
        return "STAGE:SUPERVISOR\nSolicitação: " + question + "\nResumo anterior: " + summary;
    }
}

