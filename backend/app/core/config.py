"""
Application configuration loaded from environment variables.
Uses Pydantic Settings for validation and type safety.
"""

from pydantic_settings import BaseSettings
from pydantic import Field
from functools import lru_cache


class Settings(BaseSettings):
    # ── App ──────────────────────────────────────────────────────────────
    app_name: str = "ByteBudd"
    app_version: str = "1.0.0"
    environment: str = Field(default="development", alias="ENVIRONMENT")
    debug: bool = Field(default=False)

    # ── Database (internal Postgres) ─────────────────────────────────────
    database_url: str = Field(
        default="postgresql+asyncpg://bytebudd:bytebudd_secret@localhost:5432/bytebudd",
        alias="DATABASE_URL",
    )

    # ── Auth ─────────────────────────────────────────────────────────────
    secret_key: str = Field(
        default="change-me-in-production-use-strong-secret",
        alias="SECRET_KEY",
    )
    algorithm: str = Field(default="HS256", alias="ALGORITHM")
    access_token_expire_minutes: int = Field(
        default=60, alias="ACCESS_TOKEN_EXPIRE_MINUTES"
    )

    # ── Encryption (for stored DB passwords) ─────────────────────────────
    encryption_key: str = Field(
        default="change-me-32-char-encryption-key!",
        alias="ENCRYPTION_KEY",
    )

    # ── Ollama ───────────────────────────────────────────────────────────
    ollama_base_url: str = Field(
        default="http://host.docker.internal:11434", alias="OLLAMA_BASE_URL"
    )
    ollama_model: str = Field(
        default="qwen2.5-coder:8b", alias="OLLAMA_MODEL"
    )
    ollama_timeout: int = Field(default=120, alias="OLLAMA_TIMEOUT")

    # ── CORS ─────────────────────────────────────────────────────────────
    cors_origins: str = Field(
        default="http://localhost,http://localhost:3000",
        alias="CORS_ORIGINS",
    )

    @property
    def cors_origins_list(self) -> list[str]:
        return [o.strip() for o in self.cors_origins.split(",")]

    model_config = {"env_file": ".env", "extra": "ignore"}


@lru_cache()
def get_settings() -> Settings:
    """Return cached settings instance."""
    return Settings()


settings = get_settings()
