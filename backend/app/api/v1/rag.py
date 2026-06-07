"""
RAG (Retrieval-Augmented Generation) API endpoints.
Routes queries through the Go RAG service with intelligent SQL classification.
"""

import json
import logging
from typing import AsyncIterator

from fastapi import APIRouter, Depends, HTTPException, UploadFile, File
from fastapi.responses import StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import get_db
from app.core.deps import get_current_user
from app.models.user import User
from app.services.rag_proxy import rag_proxy

router = APIRouter()
logger = logging.getLogger(__name__)


def _sse(event: str, data: dict) -> str:
    """Format a Server-Sent Event string."""
    return f"event: {event}\ndata: {json.dumps(data)}\n\n"


@router.post("/query/stream")
async def rag_query_stream(
    request: dict,
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    RAG query endpoint with intelligent routing.
    
    1. Sends the question to the Go RAG service
    2. If is_sql_query=True → yields an event to switch to SQL mode
    3. If is_sql_query=False → returns the RAG answer with sources
    """
    question = request.get("query", "")
    if not question:
        raise HTTPException(status_code=400, detail="Missing 'query' field")

    async def event_stream() -> AsyncIterator[str]:
        try:
            # Call RAG service
            result = await rag_proxy.query(question)
            
            is_sql = result.get("is_sql_query", False)
            
            if is_sql:
                # Tell frontend to switch to SQL mode
                yield _sse("is_sql", {"message": "Routing to SQL handler..."})
                # The frontend should now call the regular /api/v1/query/stream endpoint
            else:
                # Return RAG answer
                answer = result.get("answer", "I couldn't find an answer to your question.")
                sources = result.get("sources", [])
                
                yield _sse("thinking", {"message": "Searching documents..."})
                yield _sse("answer", {
                    "answer": answer,
                    "sources": sources,
                })
                yield _sse("done", {"message": "Done"})
                
        except Exception as e:
            logger.exception("RAG query error: %s", e)
            yield _sse("error", {"message": f"RAG service error: {str(e)}"})

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
            "Connection": "keep-alive",
        },
    )


@router.post("/upload")
async def rag_upload_file(
    file: UploadFile,
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """Upload a file (PDF/TXT/DOCX) to the RAG service for processing."""
    try:
        file_data = await file.read()
        
        if len(file_data) > 50 * 1024 * 1024:  # 50MB limit
            raise HTTPException(status_code=400, detail="File too large. Maximum size is 50MB.")
        
        result = await rag_proxy.upload_file(file_data, file.filename)
        
        return {
            "success": result.get("success", True),
            "file_id": result.get("file_id", ""),
            "filename": result.get("filename", file.filename),
            "chunks_count": result.get("chunks_count", 0),
            "message": result.get("message", "File processed successfully"),
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.exception("File upload error: %s", e)
        raise HTTPException(status_code=500, detail=f"Upload failed: {str(e)}")


@router.get("/health")
async def rag_health_check(
    current_user: User = Depends(get_current_user),
):
    """Check if the RAG service is reachable."""
    try:
        async with rag_proxy.__class__(rag_proxy.base_url) as client:
            resp = await client.get(f"{rag_proxy.base_url}/health")
            resp.raise_for_status()
            return {"rag_service": "healthy", "url": rag_proxy.base_url}
    except Exception as e:
        raise HTTPException(
            status_code=503,
            detail=f"RAG service unreachable: {str(e)}",
        )