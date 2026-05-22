"""Pydantic schemas for Ollama profile management."""

from datetime import datetime
from typing import Optional
from pydantic import BaseModel, Field, ConfigDict, field_validator
import re


def _validate_http_url(v: str) -> str:
    """Validate that the value is a valid HTTP or HTTPS URL."""
    if not re.match(r"^https?://", v, re.IGNORECASE):
        raise ValueError("URL must start with http:// or https://")
    return v.rstrip("/")


class FetchModelsRequest(BaseModel):
    host_url: str = Field(..., description="Ollama host URL, e.g. http://192.168.1.99:11434")

    @field_validator("host_url")
    @classmethod
    def validate_host_url(cls, v: str) -> str:
        return _validate_http_url(v)


class FetchModelsResponse(BaseModel):
    models: list[str]


class OllamaProfileCreate(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    host_url: str = Field(..., description="Ollama host URL")
    models: list[str] = Field(..., min_length=1, description="At least one model must be selected")

    @field_validator("host_url")
    @classmethod
    def validate_host_url(cls, v: str) -> str:
        return _validate_http_url(v)

    @field_validator("models")
    @classmethod
    def validate_models(cls, v: list[str]) -> list[str]:
        if not v:
            raise ValueError("At least one model must be selected")
        return v


class OllamaProfileUpdate(BaseModel):
    name: Optional[str] = Field(None, min_length=1, max_length=100)
    host_url: Optional[str] = None
    models: Optional[list[str]] = None

    @field_validator("host_url")
    @classmethod
    def validate_host_url(cls, v: Optional[str]) -> Optional[str]:
        if v is not None:
            return _validate_http_url(v)
        return v

    @field_validator("models")
    @classmethod
    def validate_models(cls, v: Optional[list[str]]) -> Optional[list[str]]:
        if v is not None and len(v) == 0:
            raise ValueError("At least one model must be selected")
        return v


class OllamaProfileOut(BaseModel):
    id: int
    name: str
    host_url: str
    models: list[str]
    is_active: bool
    created_at: datetime
    updated_at: datetime

    model_config = ConfigDict(from_attributes=True)


class ActivateRequest(BaseModel):
    is_active: bool
