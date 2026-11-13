package lab.trilha.guardrail;

import dev.langchain4j.data.message.AiMessage;
import dev.langchain4j.guardrail.OutputGuardrail;
import dev.langchain4j.guardrail.OutputGuardrailResult;
import io.helidon.service.registry.Service;

/** Limita o resumo ao tamanho aceito como entrada no próximo turno. */
@Service.Singleton
public class SummaryGuardrail implements OutputGuardrail {
    private final AnswerGuardrail answerRules = new AnswerGuardrail();

    @Override
    public OutputGuardrailResult validate(AiMessage message) {
        OutputGuardrailResult result = answerRules.validate(message);
        if (result.isFatal()) {
            return result;
        }
        String text = message.text();
        if (text != null && text.codePointCount(0, text.length()) > 1500) {
            return fatal("Resumo excede 1500 pontos de código Unicode.");
        }
        return result;
    }
}
