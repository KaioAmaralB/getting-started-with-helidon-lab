package lab.trilha.mcp;

import io.helidon.extensions.mcp.server.Mcp;
import io.helidon.extensions.mcp.server.McpToolResult;
import io.helidon.json.binding.Json;
import io.helidon.json.binding.JsonBinding;
import io.helidon.json.schema.JsonSchema;

import lab.trilha.domain.OrderCatalog;

/** Adaptador MCP da mesma consulta usada pelas tools locais. */
@Mcp.Server("trilha-orders")
@Mcp.Path("/orders")
@Mcp.Version("1.0.0")
public class OrderMcpServer {
    // A resposta contém um pedido ou um erro do catálogo.
    static final String RESULT_SCHEMA = """
            {"type":"object","oneOf":[
              {"type":"object","title":"Estado de um pedido fictício da Trilha",
               "properties":{"orderId":{"type":"string"},"item":{"type":"string"},
                             "status":{"type":"string"},"deliveryEstimate":{"type":"string"}},
               "required":["orderId","item","status","deliveryEstimate"],"additionalProperties":false},
              {"type":"object","title":"Consulta negada pela política da Trilha",
               "properties":{"error":{"type":"string","enum":["INVALID_ORDER_ID","ORDER_NOT_ALLOWED"]}},
               "required":["error"],"additionalProperties":false}
            ]}
            """;
    private static final JsonBinding JSON = JsonBinding.create();
    private final OrderCatalog catalog = new OrderCatalog();

    @Mcp.Tool(value = "Consulta um pedido fictício. IDs permitidos: TR-1001 ou TR-1002.",
              readOnlyHint = true, destructiveHint = false, idempotentHint = true, openWorldHint = false)
    @Mcp.ToolOutputSchemaText(RESULT_SCHEMA)
    public McpToolResult findOrder(@Mcp.Required @Mcp.Description("Identificador do pedido: TR-1001 ou TR-1002") String orderId) {
        // Valida a consulta também para chamadas diretas ao servidor MCP.
        String content = catalog.queryOrder(orderId);
        // Desserializa apenas o JSON produzido pelo catálogo.
        Object result = content.startsWith("{\"error\":")
                ? JSON.deserialize(content, OrderError.class)
                : JSON.deserialize(content, OrderSnapshot.class);
        return McpToolResult.builder().addTextContent(content).structuredContent(result).build();
    }

    /** Dados de um pedido retornado pelo catálogo. */
    @Json.Entity
    @JsonSchema.Schema
    @JsonSchema.Title("Estado de um pedido fictício da Trilha")
    @JsonSchema.Object.AdditionalProperties(false)
    public record OrderSnapshot(
            @JsonSchema.Required @JsonSchema.Description("Identificador do pedido consultado") String orderId,
            @JsonSchema.Required @JsonSchema.Description("Item fictício comprado") String item,
            @JsonSchema.Required @JsonSchema.Description("IN_TRANSIT ou DELIVERED") String status,
            @JsonSchema.Required @JsonSchema.Description("Estimativa didática ou indicação de entrega concluída") String deliveryEstimate) {
    }

    @Json.Entity
    @JsonSchema.Schema
    @JsonSchema.Title("Consulta negada pela política da Trilha")
    @JsonSchema.Object.AdditionalProperties(false)
    public record OrderError(
            @JsonSchema.Required @JsonSchema.Description("INVALID_ORDER_ID ou ORDER_NOT_ALLOWED") String error) {
    }
}
