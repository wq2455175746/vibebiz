from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from opentelemetry.sdk.trace import TracerProvider

from app.config import Settings, load_settings
from app.errors import WorkspaceError
from app.observability import configure_observability
from app.response import envelope
from app.routers import chat, files, preview, projects
from app.services.agent import build_agent
from app.services.versioning import Versioning
from app.services.workspace import Workspace


def create_app(
    settings: Settings | None = None,
    tracer_provider: TracerProvider | None = None,
) -> FastAPI:
    settings = settings or load_settings()

    app = FastAPI(title="vibebiz-sandbox", version="0.1.0")
    app.state.settings = settings
    app.state.workspace = Workspace(settings.workspaces_dir)
    app.state.versioning = Versioning()
    app.state.agent = build_agent(settings)

    app.include_router(projects.router)
    app.include_router(files.router)
    app.include_router(chat.router)
    app.include_router(preview.router)

    configure_observability(app, tracer_provider)

    @app.get("/")
    def root() -> dict:
        return envelope(
            {
                "service": "vibebiz-sandbox",
                "agentMode": settings.agent_mode,
                "workspaces": str(settings.workspaces_dir),
            }
        )

    @app.get("/health")
    def health() -> dict:
        return envelope({"status": "ok"})

    @app.exception_handler(WorkspaceError)
    async def workspace_error_handler(_: Request, exc: WorkspaceError) -> JSONResponse:
        return JSONResponse(
            status_code=exc.status,
            content={"code": exc.code, "message": exc.message},
        )

    @app.exception_handler(RequestValidationError)
    async def validation_error_handler(_: Request, exc: RequestValidationError) -> JSONResponse:
        return JSONResponse(
            status_code=400,
            content={"code": 40000, "message": "invalid request", "result": exc.errors()},
        )

    return app


app = create_app()
