"""
Query endpoint — the main feature of ByteBudd.
Accepts a natural language question and streams back SSE events.
"""

import asyncio
import json
import logging
from typing import AsyncIterator

from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from app.core.database import get_db
from app.core.deps import get_current_user
from app.llm.ollama_client import ollama_client
from app.models.user import User
from app.models.conversation import Conversation
from app.schemas.conversation import QueryRequest
from app.services.query_pipeline import run_query_pipeline

router = APIRouter()
logger = logging.getLogger(__name__)


@router.post("/stream")
async def query_stream(
    payload: QueryRequest,
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    Main query endpoint. Accepts a natural language question and streams
    back SSE events: thinking / sql / results / explanation / done / error.
    """
    # Verify the conversation belongs to this user
    result = await db.execute(
        select(Conversation).where(
            Conversation.id == payload.conversation_id,
            Conversation.user_id == current_user.id,
        )
    )
    if not result.scalar_one_or_none():
        raise HTTPException(status_code=404, detail="Conversation not found")

    async def event_stream() -> AsyncIterator[str]:
        try:
            async for chunk in run_query_pipeline(
                question=payload.question,
                conversation_id=payload.conversation_id,
                db_connection_id=payload.db_connection_id,
                user_id=current_user.id,
                db=db,
                profile_id=payload.profile_id,
                model_name=payload.model_name,
            ):
                yield chunk
        except asyncio.CancelledError:
            logger.info("Client disconnected during streaming")
        except Exception as e:
            logger.exception("Stream error: %s", e)
            yield f"event: error\ndata: {json.dumps({'message': str(e)})}\n\n"

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
            "Connection": "keep-alive",
        },
    )


@router.get("/ollama/status")
async def ollama_status(_user: User = Depends(get_current_user)):
    """Check if Ollama is reachable and the model is loaded."""
    available = await ollama_client.is_available()
    return {
        "available": available,
        "model": ollama_client.model,
        "base_url": ollama_client.base_url,
    }


@router.post("/ollama/pull")
async def pull_model(_user: User = Depends(get_current_user)):
    """Pull/download the Ollama model. Streams progress as SSE."""
    async def pull_stream():
        try:
            async for status in ollama_client.pull_model():
                yield f"data: {status}\n\n"
            yield "data: done\n\n"
        except Exception as e:
            yield f"event: error\ndata: {json.dumps({'message': str(e)})}\n\n"

    return StreamingResponse(
        pull_stream(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
