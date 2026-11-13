"""Configuração do processo por variáveis de ambiente."""
from dataclasses import dataclass, field
import os
from pathlib import Path
from urllib.parse import urlsplit


@dataclass(frozen=True)
class Settings:
    lab_root: Path
    base_url: str
    api_key: str = field(repr=False)
    routing_model: str
    expert_model: str
    mcp_url: str
    host: str
    port: int
    mcp_server_port: int
    rag_workers: int

    @classmethod
    def from_env(cls):
        root = Path(os.environ.get("LAB_ROOT", Path(__file__).resolve().parents[3])).resolve()
        return cls(
            root,
            os.environ.get("OCI_BASE_URL", "http://127.0.0.1:9000/v1").rstrip("/"),
            os.environ.get("OCI_API_KEY", "local-replay"),
            os.environ.get("OCI_ROUTING_MODEL", "trilha-replay"),
            os.environ.get("OCI_EXPERT_MODEL", "trilha-replay"),
            os.environ.get("MCP_URL", "http://127.0.0.1:8081/orders"),
            os.environ.get("HOST", "127.0.0.1"),
            int(os.environ.get("PORT", "8082")),
            int(os.environ.get("MCP_SERVER_PORT", "8083")),
            int(os.environ.get("RAG_WORKERS", "256")),
        )

    @property
    def provider_mode(self):
        return "local" if urlsplit(self.base_url).hostname in ("localhost", "127.0.0.1") else "remote"
