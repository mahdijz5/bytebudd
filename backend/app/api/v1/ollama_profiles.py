"""
Ollama Profiles API — admin CRUD + activate/deactivate + model fetching.
Regular users can GET /active to see profiles available for query selection.
"""

import logging
from datetime import datetime, timezone

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import get_db
from app.core.deps import get_current_user, get_admin_user
from app.core.config import settings
from app.models.user import User
from app.schemas.ollama_profile import (
    FetchModelsRequest,
    FetchModelsResponse,
    OllamaProfileCreate,
    OllamaProfileUpdate,
    OllamaProfileOut,
    ActivateRequest,
)
from app.services.ollama_profile_service import (
    fetch_models_from_host,
    list_profiles,
    get_profile,
    create_profile,
    update_profile,
    delete_profile,
    set_active,
    get_active_profiles,
    ProfileNotFoundError,
    ProfileNameConflictError,
    HostUnreachableError,
    InvalidHostResponseError,
)

router = APIRouter()
logger = logging.getLogger(__name__)


def _handle_service_error(exc: Exception) -> None:
    """Map service-layer exceptions to appropriate HTTP responses."""
    if isinstance(exc, ProfileNotFoundError):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail=str(exc))
    if isinstance(exc, ProfileNameConflictError):
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=str(exc))
    if isinstance(exc, (HostUnreachableError, InvalidHostResponseError)):
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail=str(exc))
    raise exc


@router.post("/fetch-models", response_model=FetchModelsResponse)
async def fetch_models(
    payload: FetchModelsRequest,
    _admin: User = Depends(get_admin_user),
):
    """Probe an Ollama host and return its available models. Admin only."""
    try:
        models = await fetch_models_from_host(payload.host_url)
        return FetchModelsResponse(models=models)
    except Exception as exc:
        _handle_service_error(exc)


@router.get("/active", response_model=list[OllamaProfileOut])
async def list_active_profiles(
    db: AsyncSession = Depends(get_db),
    _user: User = Depends(get_current_user),
):
    """Return all active Ollama profiles. Available to all authenticated users."""
    profiles = await get_active_profiles(db)
    return [OllamaProfileOut.model_validate(p) for p in profiles]


@router.get("/", response_model=list[OllamaProfileOut])
async def list_all_profiles(
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """
    Return all Ollama profiles. Admin only.
    Includes a synthetic 'Environment Default' entry when OLLAMA_BASE_URL is set.
    """
    profiles = await list_profiles(db)
    result = [OllamaProfileOut.model_validate(p) for p in profiles]

    if settings.ollama_base_url:
        now = datetime.now(timezone.utc)
        result.append(OllamaProfileOut(
            id=0,
            name="Environment Default",
            host_url=settings.ollama_base_url,
            models=[settings.ollama_model] if settings.ollama_model else [],
            is_active=True,
            created_at=now,
            updated_at=now,
        ))

    return result


@router.post("/", response_model=OllamaProfileOut, status_code=status.HTTP_201_CREATED)
async def create_ollama_profile(
    payload: OllamaProfileCreate,
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """Create a new Ollama profile. Admin only."""
    try:
        profile = await create_profile(db, payload)
        return OllamaProfileOut.model_validate(profile)
    except Exception as exc:
        _handle_service_error(exc)


@router.get("/{profile_id}", response_model=OllamaProfileOut)
async def get_ollama_profile(
    profile_id: int,
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """Get a single Ollama profile by ID. Admin only."""
    try:
        profile = await get_profile(db, profile_id)
        return OllamaProfileOut.model_validate(profile)
    except Exception as exc:
        _handle_service_error(exc)


@router.patch("/{profile_id}", response_model=OllamaProfileOut)
async def update_ollama_profile(
    profile_id: int,
    payload: OllamaProfileUpdate,
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """Partially update an Ollama profile. Admin only."""
    try:
        profile = await update_profile(db, profile_id, payload)
        return OllamaProfileOut.model_validate(profile)
    except Exception as exc:
        _handle_service_error(exc)


@router.delete("/{profile_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_ollama_profile(
    profile_id: int,
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """Delete an Ollama profile. Admin only."""
    try:
        await delete_profile(db, profile_id)
    except Exception as exc:
        _handle_service_error(exc)


@router.patch("/{profile_id}/active", response_model=OllamaProfileOut)
async def toggle_profile_active(
    profile_id: int,
    payload: ActivateRequest,
    db: AsyncSession = Depends(get_db),
    _admin: User = Depends(get_admin_user),
):
    """Activate or deactivate an Ollama profile. Admin only."""
    try:
        profile = await set_active(db, profile_id, payload.is_active)
        return OllamaProfileOut.model_validate(profile)
    except Exception as exc:
        _handle_service_error(exc)
