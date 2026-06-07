"""
RAG Service proxy - forwards requests to the Go RAG service.
"""

import httpx
from typing import Optional
from app.core.config import get_settings

settings = get_settings()


class RAGProxy:
    """HTTP client for the Go RAG service."""

    def __init__(self, base_url: Optional[str] = None):
        self.base_url = base_url or settings.rag_service_url.rstrip("/")

    async def query(self, question: str) -> dict:
        """
        Query the RAG service. Returns:
        - {"is_sql_query": True} if the question needs SQL
        - {"is_sql_query": False, "answer": "...", "sources": [...]} for document answers
        """
        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.post(
                f"{self.base_url}/query",
                json={"query": question},
            )
            resp.raise_for_status()
            return resp.json()

    async def upload_file(self, file_data: bytes, filename: str) -> dict:
        """Upload a file to the RAG service for processing."""
        files = {"file": (filename, file_data)}
        async with httpx.AsyncClient(timeout=300.0) as client:
            resp = await client.post(
                f"{self.base_url}/upload",
                files=files,
            )
            resp.raise_for_status()
            return resp.json()


rag_proxy = RAGProxy()