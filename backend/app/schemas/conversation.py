"""Conversation and message schemas."""

from datetime import datetime
from typing import Optional, List
from pydantic import BaseModel


class ConversationCreate(BaseModel):
    db_connection_id: int
    title: str = "New Conversation"


class ConversationOut(BaseModel):
    id: int
    title: str
    db_connection_id: Optional[int]
    created_at: datetime
    updated_at: datetime

    model_config = {"from_attributes": True}


class MessageOut(BaseModel):
    id: int
    role: str
    content: str
    generated_sql: Optional[str]
    result_data: Optional[str]
    created_at: datetime

    model_config = {"from_attributes": True}


class ConversationDetail(ConversationOut):
    messages: List[MessageOut] = []


class QueryRequest(BaseModel):
    question: str
    conversation_id: int
    db_connection_id: int
    profile_id: Optional[int] = None   # Ollama profile to use (None = env fallback)
    model_name: Optional[str] = None   # Model within the selected profile
