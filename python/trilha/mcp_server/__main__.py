import uvicorn
from trilha.assistant.config import Settings
from trilha.mcp_server.server import create_mcp_app

if __name__ == "__main__":
    settings = Settings.from_env()
    uvicorn.run(create_mcp_app(settings), host=settings.host, port=settings.mcp_server_port,
                workers=1, access_log=False, timeout_keep_alive=60)
