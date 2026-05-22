"""API v1 router."""
from fastapi import APIRouter
from app.api.v1 import auth, databases, conversations, query, ollama_profiles

router = APIRouter()
router.include_router(auth.router, prefix="/auth", tags=["auth"])
router.include_router(databases.router, prefix="/databases", tags=["databases"])
router.include_router(conversations.router, prefix="/conversations", tags=["conversations"])
router.include_router(query.router, prefix="/query", tags=["query"])
router.include_router(ollama_profiles.router, prefix="/ollama-profiles", tags=["ollama-profiles"])
