"""FastAPI application factory — parameterised for tests and production alike."""

import asyncio
import logging
import os
from collections.abc import Callable
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.adapters.registry import AdapterRegistry
from app.adapters.registry import registry as default_registry
from app.api.routes import router
from app.core.job_store import start_pruner

_logger = logging.getLogger(__name__)

Registrar = Callable[[AdapterRegistry], None]


def register_default_adapters(registry: AdapterRegistry) -> None:
    from app.adapters.builtin_adapter import BuiltinAdapter
    from app.adapters.nikto_adapter import NiktoAdapter
    from app.adapters.nuclei_adapter import NucleiAdapter
    from app.adapters.shodan_adapter import ShodanAdapter
    from app.adapters.virustotal_adapter import VirusTotalAdapter
    from app.adapters.zap_adapter import ZAPAdapter

    registry.register(BuiltinAdapter())
    registry.register(NucleiAdapter())
    registry.register(NiktoAdapter())
    registry.register(ShodanAdapter())
    registry.register(VirusTotalAdapter())
    registry.register(ZAPAdapter())


def create_app(register: Registrar | None = None) -> FastAPI:
    """Construct and return a configured `FastAPI` instance."""

    @asynccontextmanager
    async def lifespan(_app: FastAPI):
        for name in list(default_registry.available()):
            default_registry.unregister(name)
        if register is not None:
            register(default_registry)
        else:
            register_default_adapters(default_registry)

        loop = asyncio.get_running_loop()
        prune_task = start_pruner(loop)
        try:
            yield
        finally:
            prune_task.cancel()
            try:
                await prune_task
            except asyncio.CancelledError:
                pass

    app = FastAPI(
        title="moku-analyzer",
        description="Vulnerability analyzer service for the Moku platform",
        version="0.2.0",
        lifespan=lifespan,
    )

    cors_origins_raw = os.environ.get("MOKU_ANALYZER_CORS_ALLOW_ORIGINS", "")
    cors_origins = [origin for origin in cors_origins_raw.split(",") if origin.strip()]
    app.add_middleware(
        CORSMiddleware,
        allow_origins=cors_origins,
        allow_methods=["GET", "POST"],
        allow_headers=["Authorization", "Content-Type", "X-Moku-Token"],
    )

    app.include_router(router)
    return app
