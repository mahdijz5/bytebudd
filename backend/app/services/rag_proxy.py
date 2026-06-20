"""
RAG Service proxy - forwards requests to the Go RAG service.
"""

import httpx
from typing import Optional, List, Dict, Any
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

    async def upload_file(self, file_data: bytes, filename: str, user_id: int = 1, document_id: Optional[int] = None) -> dict:
        """Upload a file to the RAG service for processing."""
        files = {"file": (filename, file_data)}
        data = {"user_id": str(user_id)}
        if document_id:
            data["document_id"] = str(document_id)
        
        async with httpx.AsyncClient(timeout=300.0) as client:
            resp = await client.post(
                f"{self.base_url}/upload",
                files=files,
                data=data,
            )
            resp.raise_for_status()
            return resp.json()

    async def list_documents(self, user_id: int, limit: int = 50, offset: int = 0) -> Dict[str, Any]:
        """List documents for a user."""
        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(
                f"{self.base_url}/documents",
                params={"user_id": str(user_id), "limit": limit, "offset": offset},
            )
            resp.raise_for_status()
            return resp.json()

    async def get_document(self, document_id: int) -> Dict[str, Any]:
        """Get a single document by ID."""
        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(
                f"{self.base_url}/documents/{document_id}",
            )
            resp.raise_for_status()
            return resp.json()

    async def delete_document(self, document_id: int) -> Dict[str, Any]:
        """Delete a document and its associated data."""
        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.delete(
                f"{self.base_url}/documents/{document_id}",
            )
            resp.raise_for_status()
            return resp.json()


rag_proxy = RAGProxy()