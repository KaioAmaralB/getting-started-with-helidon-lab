package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.service.SystemMessage;
import dev.langchain4j.service.UserMessage;
import dev.langchain4j.service.V;
import dev.langchain4j.service.guardrail.InputGuardrails;
import dev.langchain4j.service.guardrail.OutputGuardrails;
import lab.trilha.guardrail.RequestGuardrail;
import lab.trilha.guardrail.AnswerGuardrail;

/** AI Service de atendimento, sem composição de agentes. */
@Ai.Service(value = "policy-service", autoDiscovery = false)
@Ai.ChatModel("expert-model")
@Ai.ContentRetriever("returns")
@Ai.Tools(OrderTools.class)
public interface PolicyService {
    @SystemMessage("STAGE:RETURN\n" + Prompts.EXPERT)
    @UserMessage("Solicitação: {{question}}\nResumo anterior: {{previousSummary}}")
    @InputGuardrails(RequestGuardrail.class)
    @OutputGuardrails(AnswerGuardrail.class)
    String answer(@V("question") String question, @V("previousSummary") String previousSummary);
}
