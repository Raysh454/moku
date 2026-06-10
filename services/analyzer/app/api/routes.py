"""HTTP routes for the moku-analyzer service."""

from fastapi import APIRouter, Depends, HTTPException, Query, status

from app.adapters.registry import registry
from app.api.security import require_internal_token
from app.core.job_store import JobStoreFull, job_store
from app.core.runner import submit_scan_job
from app.models.schemas import (
    Backend,
    Capabilities,
    HealthResponse,
    HealthStatus,
    ScanRequest,
    ScanResult,
    SubmitResponse,
)

router = APIRouter(dependencies=[Depends(require_internal_token)])


@router.post(
    "/scan",
    response_model=SubmitResponse,
    status_code=status.HTTP_202_ACCEPTED,
    response_model_by_alias=True,
)
async def submit_scan(request: ScanRequest) -> SubmitResponse:
    backend_name = _backend_name(request.backend)
    if not registry.has(backend_name):
        raise HTTPException(
            status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
            detail=f"backend {backend_name!r} is not registered",
        )
    try:
        job_id = job_store.create(request)
    except JobStoreFull as exc:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail=str(exc),
        ) from exc
    submit_scan_job(job_id)
    return SubmitResponse(job_id=job_id)


def _backend_name(backend) -> str:
    """`use_enum_values=True` may surface a plain string; normalise either form."""
    if isinstance(backend, Backend):
        return backend.value
    return str(backend)


@router.get(
    "/scan/{job_id}",
    response_model=ScanResult,
    response_model_by_alias=True,
)
async def get_scan(job_id: str) -> ScanResult:
    result = job_store.get(job_id)
    if result is None:
        raise HTTPException(status_code=404, detail="job not found")
    return result


@router.get(
    "/health",
    response_model=HealthResponse,
    response_model_by_alias=True,
)
async def health() -> HealthResponse:
    return HealthResponse(
        status=HealthStatus.OK,
        backend=None,
        adapters_available=registry.available(),
    )


@router.get(
    "/capabilities",
    response_model=Capabilities,
    response_model_by_alias=True,
)
async def capabilities(backend: Backend = Query(...)) -> Capabilities:
    name = _backend_name(backend)
    if not registry.has(name):
        raise HTTPException(status_code=404, detail="backend not registered")
    adapter = registry.get(name)
    return adapter.capabilities()
