package lab.trilha.http;

import java.util.function.Supplier;
import io.helidon.http.Http;
import io.helidon.config.Config;
import java.net.URI;
import io.helidon.http.HttpException;
import io.helidon.http.Status;
import io.helidon.service.registry.Service;
import io.helidon.webserver.http.RestServer;
import lab.trilha.ai.*;
import lab.trilha.metrics.ModelMetrics;
import lab.trilha.guardrail.RequestGuardrail;
import lab.trilha.guardrail.AnswerGuardrail;
import dev.langchain4j.data.message.AiMessage;
import dev.langchain4j.data.message.UserMessage;
import lab.trilha.http.Messages.*;

@RestServer.Endpoint
@Http.Path("/")
@Service.Singleton
public class SupportEndpoint {
    private final SupportWorkflow workflow;
    private final SupervisorWorkflow supervisor;
    private final McpWorkflow mcp;
    private final ModelMetrics metrics;
    private final PolicyService direct;
    private final String providerMode;

    @Service.Inject
    SupportEndpoint(SupportWorkflow workflow, SupervisorWorkflow supervisor, McpWorkflow mcp, ModelMetrics metrics, PolicyService direct, Config config) {
        this.workflow = workflow;
        this.supervisor = supervisor;
        this.mcp = mcp;
        this.metrics = metrics;
        this.direct = direct;
        String host = URI.create(config.get("langchain4j.providers.open-ai.base-url").asString().get()).getHost();
        this.providerMode = ("127.0.0.1".equals(host) || "localhost".equals(host)) ? "local" : "remote";
    }

    @Http.GET @Http.Path("/health") @Http.Produces("application/json")
    public Health health() { return new Health("UP"); }

    @Http.GET @Http.Path("/api/v1/runtime") @Http.Produces("application/json")
    public RuntimeInfo runtime() { return new RuntimeInfo(System.getProperty("java.version"), Thread.currentThread().isVirtual(), providerMode); }

    @Http.GET @Http.Path("/api/v1/metrics") @Http.Produces("application/json")
    public ModelMetrics.Snapshot metrics() { return metrics.snapshot(); }

    @Http.POST @Http.Path("/api/v1/workflow") @Http.Consumes("application/json") @Http.Produces("application/json")
    public Response workflow(@Http.Entity Request request) {
        Request valid = validate(request);
        return invoke(() -> workflow.chat(valid.message(), valid.summary()));
    }

    @Http.POST @Http.Path("/api/v1/supervisor") @Http.Consumes("application/json") @Http.Produces("application/json")
    public Response supervisor(@Http.Entity Request request) {
        Request valid = validate(request);
        return invoke(() -> {
            Response response = supervisor.chat(valid.message(), valid.summary());
            // Valida também a síntese produzida pelo supervisor.
            if (new AnswerGuardrail().validate(AiMessage.from(response.answer())).isFatal()) {
                throw new HttpException("Resposta bloqueada pelo guardrail.", Status.UNPROCESSABLE_CONTENT_422);
            }
            return response;
        });
    }

    @Http.POST @Http.Path("/api/v1/mcp") @Http.Consumes("application/json") @Http.Produces("application/json")
    public Response mcp(@Http.Entity Request request) {
        Request valid = validate(request);
        return invoke(() -> mcp.chat(valid.message(), valid.summary()));
    }

    @Http.POST @Http.Path("/api/v1/direct") @Http.Consumes("application/json") @Http.Produces("application/json")
    public Response direct(@Http.Entity Request request) {
        Request valid = validate(request);
        // A rota direta não atualiza o resumo da conversa.
        return invoke(() -> new Response(direct.answer(valid.message(), valid.summary()), ""));
    }
    private static Request validate(Request request) {
        if (request == null || request.message() == null || request.message().isBlank()
                || request.message().codePointCount(0, request.message().length()) > 2000 || (request.summary() != null && request.summary().codePointCount(0, request.summary().length()) > 1500)) {
            throw new HttpException("message deve conter 1 a 2000 pontos de código Unicode e summary até 1500.", Status.BAD_REQUEST_400);
        }
        String summary = request.summary() == null ? "" : request.summary();
        // Verifica a mensagem e o resumo antes da primeira chamada ao modelo.
        if (new RequestGuardrail().validate(UserMessage.from(request.message() + "\n" + summary)).isFatal()) {
            throw new HttpException("Entrada bloqueada pelo guardrail.", Status.UNPROCESSABLE_CONTENT_422);
        }
        return new Request(request.message(), summary);
    }

    private static Response invoke(Supplier<Response> action) {
        try { return action.get(); }
        catch (HttpException e) { throw e; }
        catch (RuntimeException e) {
            var failure = ProviderFailure.inspect(e);
            if (failure.guardrail()) {
                throw new HttpException("Conteúdo bloqueado pelo guardrail.", Status.UNPROCESSABLE_CONTENT_422);
            }
            System.getLogger(SupportEndpoint.class.getName()).log(System.Logger.Level.WARNING,
                    "Falha no provedor ou fluxo: " + failure.safeLog());
            throw new HttpException("Não foi possível concluir a consulta ao provedor ou à ferramenta.", Status.BAD_GATEWAY_502);
        }
    }
}





