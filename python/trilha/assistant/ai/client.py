"""Cliente Chat Completions assíncrono com pool de conexões HTTP."""
import asyncio
import httpx


class ProviderError(Exception):
    def __init__(self, kind, http_status=None):
        self.kind = kind
        self.http_status = http_status
        super().__init__(kind)


class ModelClient:
    def __init__(self, settings, metrics):
        self.settings, self.metrics = settings, metrics
        self.http = httpx.AsyncClient(
            timeout=httpx.Timeout(60.0), http2=True, trust_env=False,
            limits=httpx.Limits(max_connections=None, max_keepalive_connections=None, keepalive_expiry=30.0),
        )

    async def close(self):
        await self.http.aclose()

    async def complete(self, model, messages, tools=None):
        body = {"model": model, "messages": messages, "temperature": 0.0, "max_tokens": 512, "stream": False}
        if tools:
            body["tools"] = tools
        self.metrics.requested()
        try:
            async with asyncio.timeout(60):
                response = await self.http.post(
                    self.settings.base_url + "/chat/completions", json=body,
                    headers={"Authorization": "Bearer " + self.settings.api_key},
                )
            if response.status_code >= 400:
                raise ProviderError("HttpError", response.status_code)
            result = response.json()
            if not result.get("choices") or not isinstance(result["choices"][0].get("message"), dict):
                raise ProviderError("InvalidResponse")
            self.metrics.responded(result)
            return result["choices"][0]["message"]
        except ProviderError:
            self.metrics.failed()
            raise
        except (httpx.HTTPError, TimeoutError, ValueError, KeyError, TypeError) as error:
            self.metrics.failed()
            raise ProviderError(type(error).__name__) from None
