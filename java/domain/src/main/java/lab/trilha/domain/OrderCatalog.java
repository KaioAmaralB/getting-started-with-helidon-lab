package lab.trilha.domain;

import java.util.Map;
import java.util.regex.Pattern;

/**
 * Catálogo fictício, somente leitura, usado pelas tools locais e pelo servidor MCP.
 */
public final class OrderCatalog {
    private static final Pattern ORDER_ID = Pattern.compile("TR-\\d{4}");
    private static final Map<String, String> ORDERS = Map.of(
            "TR-1001", "{\"orderId\":\"TR-1001\",\"item\":\"barraca\",\"status\":\"IN_TRANSIT\","
                    + "\"deliveryEstimate\":\"até 3 dias úteis\"}",
            "TR-1002", "{\"orderId\":\"TR-1002\",\"item\":\"mochila\",\"status\":\"DELIVERED\","
                    + "\"deliveryEstimate\":\"entregue\"}");

    /**
     * Converte rejeições esperadas em respostas JSON, sem incluir o identificador recebido.
     */
    public String queryOrder(String orderId) {
        try {
            return findOrder(orderId);
        } catch (IllegalArgumentException e) {
            return "{\"error\":\"INVALID_ORDER_ID\"}";
        } catch (SecurityException e) {
            return "{\"error\":\"ORDER_NOT_ALLOWED\"}";
        }
    }

    /**
     * Consulta um dos pedidos permitidos no catálogo.
     *
     * @param orderId identificador exato, como TR-1001
     * @return JSON com os dados do pedido
     * @throws IllegalArgumentException se o identificador não seguir TR- seguido de quatro dígitos ASCII
     * @throws SecurityException se o identificador não estiver na lista permitida
     */
    public String findOrder(String orderId) {
        if (orderId == null || !ORDER_ID.matcher(orderId).matches()) {
            throw new IllegalArgumentException("orderId deve seguir TR- seguido de quatro dígitos (ex.: TR-1001).");
        }
        String order = ORDERS.get(orderId);
        if (order == null) {
            throw new SecurityException("Pedido fora da lista permitida do laboratório.");
        }
        return order;
    }
}
