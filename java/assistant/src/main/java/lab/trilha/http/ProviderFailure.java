package lab.trilha.http;

import java.util.Collections;
import java.util.IdentityHashMap;
import java.util.Objects;
import java.util.Set;
import java.util.StringJoiner;

import dev.langchain4j.guardrail.GuardrailException;

/** Extrai apenas metadados de falha; mensagens podem conter prompts, respostas ou credenciais. */
final class ProviderFailure {
    private ProviderFailure() { }

    static Details inspect(Throwable failure) {
        Objects.requireNonNull(failure);
        Set<Throwable> visited = Collections.newSetFromMap(new IdentityHashMap<>());
        var classes = new StringJoiner(" -> ");
        Integer httpStatus = null;
        boolean guardrail = false;
        for (Throwable cause = failure; cause != null && visited.add(cause); cause = cause.getCause()) {
            classes.add(cause.getClass().getSimpleName());
            guardrail |= cause instanceof GuardrailException;
            if (cause instanceof dev.langchain4j.exception.HttpException http) {
                httpStatus = http.statusCode();
            } else if (cause instanceof io.helidon.http.HttpException http) {
                httpStatus = http.status().code();
            }
        }
        return new Details(classes.toString(), httpStatus, guardrail);
    }

    /** Não guarda a exceção, seu texto nem seu stack trace. */
    record Details(String causeChain, Integer httpStatus, boolean guardrail) {
        String safeLog() {
            return "causas=" + causeChain + "; httpStatus=" + (httpStatus == null ? "indisponivel" : httpStatus);
        }
    }
}
