package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.declarative.ActivationCondition;
import dev.langchain4j.agentic.declarative.ConditionalAgent;
import dev.langchain4j.service.V;

@Ai.Agent("router")
public interface RouterAgent {
    @ConditionalAgent(subAgents = {OrderAgent.class, ReturnAgent.class}, outputKey = "answer")
    String route(@V("question") String question);

    @ActivationCondition(OrderAgent.class)
    static boolean order(@V("intent") Intent intent) { return intent == Intent.ORDER; }

    @ActivationCondition(ReturnAgent.class)
    static boolean returns(@V("intent") Intent intent) { return intent == Intent.RETURN; }
}
