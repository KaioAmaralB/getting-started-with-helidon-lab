"""Servidor MCP para consulta de pedidos."""
import json
import uuid

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, Response

from trilha.assistant.ai.prompts import PromptContract
from trilha.assistant.config import Settings
from trilha.domain.catalog import query_order

RESULT_SCHEMA = {
    "type": "object", "oneOf": [
        {"type": "object", "title": "Estado de um pedido fictício da Trilha",
         "properties": {"orderId": {"type": "string"}, "item": {"type": "string"},
                        "status": {"type": "string"}, "deliveryEstimate": {"type": "string"}},
         "required": ["orderId", "item", "status", "deliveryEstimate"], "additionalProperties": False},
        {"type": "object", "title": "Consulta negada pela política da Trilha",
         "properties": {"error": {"type": "string", "enum": ["INVALID_ORDER_ID", "ORDER_NOT_ALLOWED"]}},
         "required": ["error"], "additionalProperties": False},
    ],
}


def create_mcp_app(settings=None):
    settings = settings or Settings.from_env()
    contract = PromptContract(settings.lab_root)
    function = contract.data["tools"]["mcp"]["function"]
    tool = {
        "name": function["name"], "description": function["description"],
        "inputSchema": function["parameters"], "outputSchema": RESULT_SCHEMA,
        "annotations": {"title": "", "readOnlyHint": True, "destructiveHint": False,
                        "idempotentHint": True, "openWorldHint": False},
    }
    sessions = set()
    app = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)

    @app.get("/health")
    async def health():
        return {"status": "UP"}

    @app.post("/orders")
    async def orders(request: Request):
        try:
            rpc = await request.json()
            if not isinstance(rpc, dict) or rpc.get("jsonrpc") != "2.0":
                raise ValueError()
        except (ValueError, UnicodeDecodeError):
            return JSONResponse({"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "Parse error"}}, status_code=400)
        identifier, method = rpc.get("id"), rpc.get("method")
        headers = {}
        if method == "initialize":
            session = str(uuid.uuid4())
            sessions.add(session)
            headers["Mcp-Session-Id"] = session
            result = {"protocolVersion": "2025-06-18", "capabilities": {"tools": {"listChanged": False}},
                      "serverInfo": {"name": "trilha-orders", "version": "1.0.0"}}
        else:
            session = request.headers.get("mcp-session-id")
            if session not in sessions:
                return JSONResponse({"jsonrpc": "2.0", "id": identifier, "error": {"code": -32000, "message": "Session not found"}}, status_code=404)
            if method == "notifications/initialized":
                return Response(status_code=202)
            if method == "ping":
                result = {}
            elif method == "tools/list":
                result = {"tools": [tool]}
            elif method == "tools/call":
                params = rpc.get("params") or {}
                arguments = params.get("arguments") or {}
                if params.get("name") != "findOrder" or not isinstance(arguments, dict):
                    return JSONResponse({"jsonrpc": "2.0", "id": identifier, "error": {"code": -32602, "message": "Invalid params"}})
                text = query_order(arguments.get("orderId"))
                result = {"content": [{"type": "text", "text": text}], "structuredContent": json.loads(text), "isError": False}
            else:
                return JSONResponse({"jsonrpc": "2.0", "id": identifier, "error": {"code": -32601, "message": "Method not found"}})
        return JSONResponse({"jsonrpc": "2.0", "id": identifier, "result": result}, headers=headers)

    @app.delete("/orders")
    async def delete(request: Request):
        sessions.discard(request.headers.get("mcp-session-id"))
        return Response(status_code=200)

    return app
