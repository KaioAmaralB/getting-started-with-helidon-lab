import uvicorn
from trilha.assistant.config import Settings
from trilha.assistant.http.app import create_app

if __name__ == "__main__":
    settings = Settings.from_env()
    uvicorn.run(create_app(settings), host=settings.host, port=settings.port, workers=1,
                access_log=False, timeout_keep_alive=60)
