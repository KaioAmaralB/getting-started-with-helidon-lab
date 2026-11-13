package lab.trilha.guardrail;

import java.util.Locale;
import dev.langchain4j.data.message.UserMessage;
import dev.langchain4j.guardrail.InputGuardrail;
import dev.langchain4j.guardrail.InputGuardrailResult;
import io.helidon.service.registry.Service;

@Service.Singleton
public class RequestGuardrail implements InputGuardrail {
    @Override
    public InputGuardrailResult validate(UserMessage message) {
        String text = message.singleText().toLowerCase(Locale.ROOT);
        if (text.contains("-----begin private key-----") || text.contains("-----begin rsa private key-----")) {
            return fatal("Não envie chaves privadas ao laboratório.");
        }
        return success();
    }
}
