"""
Ollama Profile service — business logic for managing Ollama profiles.

Handles CRUD, host connectivity checks, model fetching, and query routing.
"""

import logging
from datetime import datetime, timezone

import httpx
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from app.core.config import settings
from app.models.ollama_profile import OllamaProfile
from app.schemas.ollama_profile import OllamaProfileCreate, OllamaProfileUpdate

logger = logging.getLogger(__name__)

# ── Custom exceptions ─────────────────────────────────────────────────────


class ProfileNotFoundError(Exception):
    """Raised when a requested profile does not exist."""
    pass


class ProfileNameConflictError(Exception):
    """Raised when a profile name is already taken."""
    pass


class HostUnreachableError(Exception):
    """Raised when the Ollama host cannot be reached within the timeout."""
    pass


class InvalidHostResponseError(Exception):
    """Raised when the Ollama host returns an unexpected response."""
    pass


# ── Model fetching ────────────────────────────────────────────────────────

async def fetch_models_from_host(host_url: str) -> list[str]:
    """
    Query {host_url}/api/tags and return the list of model names.

    Raises:
        HostUnreachableError: on timeout or connection failure
        InvalidHostResponseError: on non-200 status or malformed JSON
    """
    url = f"{host_url.rstrip('/')}/api/tags"
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.get(url)
    except httpx.TimeoutException:
        raise HostUnreachableError("Host unreachable: connection timed out")
    except Exception as e:
        raise HostUnreachableError(f"Host unreachable: {e}")

    if response.status_code != 200:
        raise InvalidHostResponseError(
            f"Host returned HTTP {response.status_code}"
        )

    try:
        data = response.json()
        model_names = [m["name"] for m in data.get("models", [])]
        return model_names
    except Exception:
        raise InvalidHostResponseError("Invalid response from host")


# ── CRUD operations ───────────────────────────────────────────────────────

async def list_profiles(db: AsyncSession) -> list[OllamaProfile]:
    """Return all profiles ordered by created_at descending."""
    result = await db.execute(
        select(OllamaProfile).order_by(OllamaProfile.created_at.desc())
    )
    return list(result.scalars().all())


async def get_profile(db: AsyncSession, profile_id: int) -> OllamaProfile:
    """Return a single profile by ID, or raise ProfileNotFoundError."""
    result = await db.execute(
        select(OllamaProfile).where(OllamaProfile.id == profile_id)
    )
    profile = result.scalar_one_or_none()
    if not profile:
        raise ProfileNotFoundError(f"Profile {profile_id} not found")
    return profile


async def create_profile(
    db: AsyncSession, data: OllamaProfileCreate
) -> OllamaProfile:
    """
    Create a new Ollama profile.

    Raises:
        ProfileNameConflictError: if the name is already taken
    """
    # Check name uniqueness
    existing = await db.execute(
        select(OllamaProfile).where(OllamaProfile.name == data.name)
    )
    if existing.scalar_one_or_none():
        raise ProfileNameConflictError("Profile name already exists")

    profile = OllamaProfile(
        name=data.name,
        host_url=data.host_url,
        models=data.models,
        is_active=False,
    )
    db.add(profile)
    await db.flush()
    await db.refresh(profile)
    return profile


async def update_profile(
    db: AsyncSession, profile_id: int, data: OllamaProfileUpdate
) -> OllamaProfile:
    """
    Partially update an existing profile.

    If host_url changes, re-fetches available models from the new host.
    Raises:
        ProfileNotFoundError: if profile doesn't exist
        ProfileNameConflictError: if new name is taken by another profile
        HostUnreachableError / InvalidHostResponseError: if host re-fetch fails
    """
    profile = await get_profile(db, profile_id)

    # Check name uniqueness if renaming
    if data.name is not None and data.name != profile.name:
        existing = await db.execute(
            select(OllamaProfile).where(
                OllamaProfile.name == data.name,
                OllamaProfile.id != profile_id,
            )
        )
        if existing.scalar_one_or_none():
            raise ProfileNameConflictError("Profile name already exists")
        profile.name = data.name

    # If host_url changed, re-fetch models
    if data.host_url is not None and data.host_url != profile.host_url:
        # This may raise HostUnreachableError or InvalidHostResponseError
        available_models = await fetch_models_from_host(data.host_url)
        profile.host_url = data.host_url
        # If caller also provided new models, use those; otherwise clear to available
        if data.models is not None:
            profile.models = data.models
        else:
            profile.models = available_models
    elif data.models is not None:
        profile.models = data.models

    profile.updated_at = datetime.now(timezone.utc)
    await db.flush()
    await db.refresh(profile)
    return profile


async def delete_profile(db: AsyncSession, profile_id: int) -> None:
    """
    Delete a profile by ID.

    Raises:
        ProfileNotFoundError: if profile doesn't exist
    """
    profile = await get_profile(db, profile_id)
    await db.delete(profile)
    await db.flush()


async def set_active(
    db: AsyncSession, profile_id: int, is_active: bool
) -> OllamaProfile:
    """
    Activate or deactivate a profile.
    Multiple profiles can be active simultaneously.

    Raises:
        ProfileNotFoundError: if profile doesn't exist
    """
    profile = await get_profile(db, profile_id)
    profile.is_active = is_active
    profile.updated_at = datetime.now(timezone.utc)
    await db.flush()
    await db.refresh(profile)
    return profile


async def get_active_profiles(db: AsyncSession) -> list[OllamaProfile]:
    """Return all profiles where is_active=True."""
    result = await db.execute(
        select(OllamaProfile)
        .where(OllamaProfile.is_active == True)  # noqa: E712
        .order_by(OllamaProfile.created_at.desc())
    )
    return list(result.scalars().all())


# ── Query routing ─────────────────────────────────────────────────────────

async def resolve_ollama_config(
    db: AsyncSession,
    profile_id: int | None,
    model_name: str | None,
) -> tuple[str, str]:
    """
    Resolve the (host_url, model_name) to use for a query.

    If profile_id and model_name are provided, look up the profile.
    Otherwise fall back to environment variable configuration.

    Returns:
        (host_url, model_name) tuple

    Raises:
        ValueError: if no configuration is available at all
    """
    if profile_id is not None and model_name is not None:
        try:
            profile = await get_profile(db, profile_id)
            return profile.host_url, model_name
        except ProfileNotFoundError:
            logger.warning(
                f"Profile {profile_id} not found, falling back to env config"
            )

    # Env-var fallback
    base_url = settings.ollama_base_url
    model = settings.ollama_model
    if not base_url or not model:
        raise ValueError(
            "No Ollama configuration available: no active profile selected "
            "and OLLAMA_BASE_URL / OLLAMA_MODEL environment variables are not set."
        )
    return base_url, model
