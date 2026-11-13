"""Cliente MCP Streamable HTTP com sessões e suporte a structuredContent."""
import asyncio
from contextlib import suppress
import json
import httpx
from trilha.assistant.ai.client import ProviderError


class McpClient:
    PROTOCOL = "2025-11-25"

    def __init__(self, url):
        self.url = url
        self.http = httpx.AsyncClient(timeout=10.0, http2=True, trust_env=False,
                                      limits=httpx.Limits(max_connections=None, max_keepalive_connections=None,
                                                         keepalive_expiry=30.0))
        self.session = None
        self._next_id = 0
        self._initialize_lock = asyncio.Lock()
        self._tool_cache = None
        self._health_task = None

    async def close(self):
        if self._health_task is not None:
            self._health_task.cancel()
            with suppress(asyncio.CancelledError):
                await self._health_task
        await self.http.aclose()

    async def _health_loop(self):
        # Mantém a cadência do DefaultMcpClient: primeiro ping após 30s, intervalo de 30s e reconexão a cada 5s.
        loop = asyncio.get_running_loop()
        deadline = loop.time() + 30
        while True:
            await asyncio.sleep(max(0, deadline - loop.time()))
            deadline += 30
            try:
                await self._send("ping")
            except ProviderError:
                self.session = None
                self._tool_cache = None
                while True:
                    await asyncio.sleep(5)
                    try:
                        await self.initialize()
                        break
                    except ProviderError:
                        continue

    async def _send(self, method, params=None, notification=False):
        payload = {"jsonrpc": "2.0", "method": method}
        if not notification:
            self._next_id += 1
            payload["id"] = self._next_id
        if params is not None:
            payload["params"] = params
        headers = {"Accept": "application/json, text/event-stream", "MCP-Protocol-Version": self.PROTOCOL}
        if self.session:
            headers["Mcp-Session-Id"] = self.session
        try:
            async with asyncio.timeout(10):
                response = await self.http.post(self.url, json=payload, headers=headers)
            if response.status_code >= 400:
                raise ProviderError("McpHttpError", response.status_code)
            if response.headers.get("mcp-session-id"):
                self.session = response.headers["mcp-session-id"]
            if notification:
                return None
            if "text/event-stream" in response.headers.get("content-type", ""):
                results = []
                for block in response.text.replace("\r\n", "\n").split("\n\n"):
                    data = "\n".join(line[5:].lstrip() for line in block.splitlines() if line.startswith("data:"))
                    if data:
                        item = json.loads(data)
                        if item.get("id") == payload["id"]:
                            results.append(item)
                if not results:
                    raise ProviderError("McpMissingResponse")
                result = results[-1]
            else:
                result = response.json()
            if result.get("id") != payload["id"] or "error" in result:
                raise ProviderError("McpRpcError")
            return result.get("result", {})
        except ProviderError:
            raise
        except (httpx.HTTPError, TimeoutError, ValueError, TypeError, KeyError) as error:
            raise ProviderError(type(error).__name__) from None

    async def initialize(self):
        if self._tool_cache is not None:
            return
        async with self._initialize_lock:
            if self._tool_cache is not None:
                return
            result = await self._send("initialize", {
                "protocolVersion": self.PROTOCOL, "capabilities": {},
                "clientInfo": {"name": "trilha-python", "version": "1.0.0"},
            })
            self.PROTOCOL = result.get("protocolVersion", self.PROTOCOL)
            await self._send("notifications/initialized", notification=True)
            listed = await self._send("tools/list", {})
            self._tool_cache = [
                {"type": "function", "function": {"name": tool["name"],
                 "description": tool.get("description", ""), "parameters": tool["inputSchema"]}}
                for tool in listed.get("tools", [])
            ]
            if self._health_task is None:
                self._health_task = asyncio.create_task(self._health_loop())

    async def tools(self):
        await self.initialize()
        return self._tool_cache

    async def call(self, name, arguments):
        await self.initialize()
        result = await self._send("tools/call", {"name": name, "arguments": arguments})
        if result.get("isError"):
            raise ProviderError("McpToolError")
        if "structuredContent" in result:
            return json.dumps(result["structuredContent"], ensure_ascii=False, separators=(",", ":"))
        texts = [part.get("text", "") for part in result.get("content", []) if part.get("type") == "text"]
        return "\n".join(texts)
