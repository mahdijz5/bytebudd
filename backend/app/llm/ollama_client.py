"""
Ollama LLM client for ByteBudd.
Handles completion requests and model management via the Ollama HTTP API.
"""

import json
import logging
from typing import AsyncIterator

import httpx

from app.core.config import settings

logger = logging.getLogger(__name__)


class OllamaError(Exception):
    """Raised when Ollama returns an error or times out."""
    pass


# Shared generation options for all requests
_GENERATION_OPTIONS = {
    "temperature": 0.1,  # Low temperature = more deterministic SQL output
    "top_p": 0.9,
    "num_predict": 512,  # SQL queries are short; no need for more tokens
}


async def _call_ollama(host_url: str, model: str, prompt: str, timeout: int) -> str:
    """
    Send a prompt to an Ollama server and return the generated text.

    This is the single shared implementation used by both the default client
    and profile-aware generation. Raises OllamaError on any failure.
    """
    payload = {
        "model": model,
        "prompt": prompt,
        "stream": False,
        "options": _GENERATION_OPTIONS,
    }
    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(
                f"{host_url.rstrip('/')}/api/generate",
                json=payload,
            )
            response.raise_for_status()
            return response.json().get("response", "").strip()
    except httpx.TimeoutException:
        raise OllamaError(f"Ollama request timed out after {timeout}s")
    except httpx.HTTPStatusError as e:
        raise OllamaError(f"Ollama HTTP error {e.response.status_code}: {e.response.text}")
    except Exception as e:
        raise OllamaError(f"Ollama connection error: {e}")


class OllamaClient:
    """Thin async wrapper around the Ollama HTTP API using env-var configuration."""

    def __init__(self):
        self.base_url = settings.ollama_base_url
        self.model = settings.ollama_model
        self.timeout = settings.ollama_timeout

    async def is_available(self) -> bool:
        """Check if Ollama is reachable and the configured model is loaded."""
        try:
            async with httpx.AsyncClient(timeout=5) as client:
                response = await client.get(f"{self.base_url}/api/tags")
                if response.status_code != 200:
                    return False
                data = response.json()
                model_names = [m["name"] for m in data.get("models", [])]
                # Match by base name to handle tags like ":latest"
                base_name = self.model.split(":")[0]
                return any(m.startswith(base_name) for m in model_names)
        except Exception:
            return False

    async def generate(self, prompt: str) -> str:
        """Generate a SQL query from a prompt using the configured model."""
        return await _call_ollama(self.base_url, self.model, prompt, self.timeout)

    async def pull_model(self) -> AsyncIterator[str]:
        """Pull/download the configured model from the Ollama registry."""
        payload = {"name": self.model, "stream": True}

        async with httpx.AsyncClient(timeout=600) as client:
            async with client.stream(
                "POST", f"{self.base_url}/api/pull", json=payload
            ) as response:
                response.raise_for_status()
                async for line in response.aiter_lines():
                    if line.strip():
                        try:
                            chunk = json.loads(line)
                            status = chunk.get("status", "")
                            if status:
                                yield status
                        except json.JSONDecodeError:
                            continue


# Singleton used by the query pipeline when no profile is selected
ollama_client = OllamaClient()


async def generate_with_profile(host_url: str, model: str, prompt: str) -> str:
    """
    Generate a SQL query using an explicit Ollama host and model.
    Used when the user has selected a named Ollama profile.
    """
    return await _call_ollama(host_url, model, prompt, settings.ollama_timeout)
