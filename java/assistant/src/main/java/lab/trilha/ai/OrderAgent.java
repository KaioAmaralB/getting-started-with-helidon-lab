package lab.trilha.ai;

import io.helidon.integrations.langchain4j.Ai;
import dev.langchain4j.agentic.Agent;
import dev.langchain4j.service.SystemMessage;
import dev.langchain4j.service.UserMessage;
import dev.langchain4j.service.V;
import dev.langchain4j.service.guardrail.InputGuardrails;
import dev.langchain4j.service.guardrail.OutputGuardrails;
import lab.trilha.guardrail.RequestGuardrail;
import lab.trilha.guardrail.AnswerGuardrail;

@Ai.Agent("OrderAgent")
@Ai.ChatModel("expert-model")
@Ai.ContentRetriever("orders")
@Ai.Tools(OrderTools.class)
public interface OrderAgent {
    @SystemMessage("STAGE:ORDER\n" + Prompts.EXPERT)
    @UserMessage("Solicitação: {{question}}\nResumo anterior: {{previousSummary}}")
    @InputGuardrails(RequestGuardrail.class)
    @OutputGuardrails(AnswerGuardrail.class)
    @Agent(value = "Consulta status e entrega de pedidos", outputKey = "answer")
    String answer(@V("question") String question, @V("previousSummary") String previousSummary);
}
