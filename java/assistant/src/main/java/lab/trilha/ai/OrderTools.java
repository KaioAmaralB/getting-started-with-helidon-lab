package lab.trilha.ai;

import io.helidon.service.registry.Service;
import lab.trilha.domain.OrderCatalog;
import dev.langchain4j.agent.tool.P;
import dev.langchain4j.agent.tool.Tool;

@Service.Singleton
public class OrderTools {
    private final OrderCatalog catalog = new OrderCatalog();

    @Tool("Consulta o status de um pedido fictício pelo identificador TR-1001 ou TR-1002. Não altera pedidos.")
    public String findOrder(@P("Identificador do pedido, por exemplo TR-1001") String orderId) {
        return catalog.queryOrder(orderId);
    }
}

