package lab.trilha.guardrail;

import java.util.Locale;
import dev.langchain4j.data.message.AiMessage;
import dev.langchain4j.guardrail.OutputGuardrail;
import dev.langchain4j.guardrail.OutputGuardrailResult;
import io.helidon.service.registry.Service;

@Service.Singleton
public class AnswerGuardrail implements OutputGuardrail {
    @Override
    public OutputGuardrailResult validate(AiMessage message) {
        // Chamadas de ferramenta podem não conter texto.
        if (message.text() == null && message.hasToolExecutionRequests()) { return success(); }
        String text = message.text();
        if (text == null || text.isBlank() || text.codePointCount(0, text.length()) > 4000) { return fatal("Resposta vazia ou excessiva."); }
        String lower = text.toLowerCase(Locale.ROOT);
        if (lower.contains("estorno realizado") || lower.contains("troca aprovada") || lower.contains("-----begin")) {
            return fatal("Resposta afirma uma operação indisponível ou contém material sensível.");
        }
        return success();
    }
}
