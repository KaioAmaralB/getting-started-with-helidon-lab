"""Endpoints HTTP do assistente com ASGI e asyncio."""
from contextlib import asynccontextmanager
import logging
import platform

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, PlainTextResponse

from trilha.assistant.config import Settings
from trilha.assistant.ai.agents import Agents
from trilha.assistant.ai.client import ModelClient, ProviderError
from trilha.assistant.ai.prompts import PromptContract
from trilha.assistant.ai.workflows import SupportWorkflows
from trilha.assistant.guardrail.rules import validate_request, InvalidRequest, GuardrailError, SupervisorOutputGuardrailError
from trilha.assistant.metrics.counters import ModelMetrics
from trilha.assistant.rag.retriever import PolicyRetriever
from trilha.mcp_server.client import McpClient

LOGGER = logging.getLogger("trilha.python")


def create_app(settings=None):
    settings = settings or Settings.from_env()

    @asynccontextmanager
    async def lifespan(app):
        contract = PromptContract(settings.lab_root)
        metrics = ModelMetrics()
        client = ModelClient(settings, metrics)
        rag = PolicyRetriever(settings)
        mcp = McpClient(settings.mcp_url)
        try:
            await mcp.initialize()
            app.state.metrics = metrics
            app.state.workflows = SupportWorkflows(Agents(settings, contract, client, rag, mcp))
            yield
        finally:
            await mcp.close()
            await client.close()
            rag.close()

    app = FastAPI(title="Trilha Python", lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)

    @app.get("/health")
    async def health():
        return {"status": "UP"}

    @app.get("/api/v1/runtime")
    async def runtime():
        return {"language": "python", "runtimeVersion": platform.python_version(), "virtualThread": False,
                "providerMode": settings.provider_mode, "embeddingWorkers": settings.rag_workers}

    @app.get("/api/v1/metrics")
    async def metrics():
        return app.state.metrics.snapshot()

    async def invoke(route, request):
        if request.headers.get("content-type", "").split(";", 1)[0].strip().lower() != "application/json":
            return PlainTextResponse("Content-Type deve ser application/json.", status_code=415)
        validated = False
        try:
            try:
                body = await request.json()
            except (ValueError, UnicodeDecodeError):
                raise InvalidRequest() from None
            message, summary = validate_request(body)
            validated = True
            result = await getattr(app.state.workflows, route)(message, summary)
            return JSONResponse(result)
        except InvalidRequest:
            return PlainTextResponse("message deve conter 1 a 2000 pontos de código Unicode e summary até 1500.", status_code=400)
        except SupervisorOutputGuardrailError:
            return PlainTextResponse("Resposta bloqueada pelo guardrail.", status_code=422)
        except GuardrailError:
            text = "Conteúdo bloqueado pelo guardrail." if validated else "Entrada bloqueada pelo guardrail."
            return PlainTextResponse(text, status_code=422)
        except ProviderError as error:
            LOGGER.warning("Falha no provedor ou fluxo: causa=%s; httpStatus=%s", error.kind,
                           error.http_status if error.http_status is not None else "indisponivel")
            return PlainTextResponse("Não foi possível concluir a consulta ao provedor ou à ferramenta.", status_code=502)
        except Exception as error:
            LOGGER.warning("Falha no provedor ou fluxo: causa=%s", type(error).__name__)
            return PlainTextResponse("Não foi possível concluir a consulta ao provedor ou à ferramenta.", status_code=502)

    @app.post("/api/v1/direct")
    async def direct(request: Request):
        return await invoke("direct", request)

    @app.post("/api/v1/workflow")
    async def workflow(request: Request):
        return await invoke("workflow", request)

    @app.post("/api/v1/supervisor")
    async def supervisor(request: Request):
        return await invoke("supervisor", request)

    @app.post("/api/v1/mcp")
    async def mcp(request: Request):
        return await invoke("mcp", request)

    return app
