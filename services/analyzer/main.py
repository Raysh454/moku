"""Entry point for `uvicorn main:app` — wires the FastAPI application together."""

import logging
import os

from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=os.environ.get("MOKU_ANALYZER_LOG_LEVEL", "INFO"),
    format="%(asctime)s %(name)s %(levelname)s %(message)s",
)

from app.app_factory import create_app, register_default_adapters  # noqa: E402

app = create_app(register=register_default_adapters)
