"""Catálogo de pedidos com contrato JSON comum aos laboratórios."""
import re

ORDERS = {
    "TR-1001": '{"orderId":"TR-1001","item":"barraca","status":"IN_TRANSIT","deliveryEstimate":"até 3 dias úteis"}',
    "TR-1002": '{"orderId":"TR-1002","item":"mochila","status":"DELIVERED","deliveryEstimate":"entregue"}',
}


def find_order(order_id):
    if not isinstance(order_id, str) or re.fullmatch(r"TR-[0-9]{4}", order_id) is None:
        raise ValueError("INVALID_ORDER_ID")
    if order_id not in ORDERS:
        raise PermissionError("ORDER_NOT_ALLOWED")
    return ORDERS[order_id]


def query_order(order_id):
    try:
        return find_order(order_id)
    except ValueError:
        return '{"error":"INVALID_ORDER_ID"}'
    except PermissionError:
        return '{"error":"ORDER_NOT_ALLOWED"}'
