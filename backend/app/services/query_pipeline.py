"""
Core query pipeline: question → schema → LLM → validate → execute → stream.

This is the heart of ByteBudd. Each step is clearly separated for readability.
SSE events are yielded as formatted strings.
"""

import json
import logging
import time
from typing import AsyncIterator

from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from app.models.db_connection import DBConnection
from app.models.conversation import Conversation
from app.models.message import Message
from app.models.audit_log import AuditLog
from app.db_connectors import get_connector
from app.llm.ollama_client import ollama_client, OllamaError, generate_with_profile
from app.prompts.sql_prompt import build_sql_prompt
from app.services.sql_guard import validate_sql, SQLGuardError
from app.services.ollama_profile_service import resolve_ollama_config

logger = logging.getLogger(__name__)


def _sse_event(event: str, data: dict) -> str:
    """Format a Server-Sent Event string."""
    return f"event: {event}\ndata: {json.dumps(data)}\n\n"


async def run_query_pipeline(
    question: str,
    conversation_id: int,
    db_connection_id: int,
    user_id: int,
    db: AsyncSession,
    profile_id: int | None = None,
    model_name: str | None = None,
) -> AsyncIterator[str]:
    """
    Full query pipeline with SSE streaming output.

    Yields SSE-formatted strings for each pipeline stage:
      - thinking: LLM is generating
      - sql: the validated SQL
      - results: query results
      - explanation: summary
      - done: pipeline complete
      - error: something went wrong
    """
    start_time = time.monotonic()
    generated_sql: str | None = None
    row_count: int | None = None
    success = True
    error_msg: str | None = None

    try:
        # ── Step 1: Load DB connection ────────────────────────────────────
        result = await db.execute(
            select(DBConnection).where(DBConnection.id == db_connection_id)
        )
        db_conn = result.scalar_one_or_none()

        if not db_conn:
            yield _sse_event("error", {"message": "Database connection not found"})
            return

        connector = get_connector(db_conn)

        # ── Step 2: Fetch schema ──────────────────────────────────────────
        yield _sse_event("thinking", {"message": "Reading database schema..."})

        try:
            schema = await connector.get_schema()
        except Exception as e:
            logger.error(f"Schema fetch error: {e}")
            yield _sse_event("error", {"message": f"Failed to read schema: {e}"})
            return

        if not schema.strip():
            yield _sse_event("error", {"message": "Database schema is empty or unreadable"})
            return

        # ── Step 3: Build prompt ──────────────────────────────────────────
        yield _sse_event("thinking", {"message": "Analyzing your question..."})

        dialect_map = {
            "postgresql": "postgresql",
            "mysql": "mysql",
            "mariadb": "mysql",
            "sqlite": "sqlite",
        }
        dialect = dialect_map.get(db_conn.db_type, "ansi")
        prompt = build_sql_prompt(question=question, schema=schema, dialect=dialect)

        # ── Step 4: Call Ollama ───────────────────────────────────────────
        yield _sse_event("thinking", {"message": "Generating SQL with AI..."})

        try:
            if profile_id is not None and model_name is not None:
                # Profile-aware path: resolve host + model from DB
                host_url, resolved_model = await resolve_ollama_config(
                    db, profile_id, model_name
                )
                raw_sql = await generate_with_profile(host_url, resolved_model, prompt)
            else:
                # Legacy env-var path
                raw_sql = await ollama_client.generate(prompt)
        except OllamaError as e:
            logger.error(f"Ollama error: {e}")
            yield _sse_event("error", {"message": f"AI model error: {e}"})
            return

        # Check for LLM-reported errors
        if raw_sql.upper().startswith("ERROR:"):
            yield _sse_event("error", {"message": raw_sql})
            return

        # ── Step 5: Validate SQL ──────────────────────────────────────────
        try:
            safe_sql = validate_sql(raw_sql, dialect=dialect)
        except SQLGuardError as e:
            yield _sse_event("error", {"message": f"SQL validation failed: {e}"})
            success = False
            error_msg = str(e)
            return

        generated_sql = safe_sql
        yield _sse_event("sql", {"sql": safe_sql})

        # ── Step 6: Execute query ─────────────────────────────────────────
        yield _sse_event("thinking", {"message": "Executing query..."})

        try:
            rows = await connector.execute_query(safe_sql)
            row_count = len(rows)
        except Exception as e:
            logger.error(f"Query execution error: {e}")
            yield _sse_event("error", {"message": f"Query execution failed: {e}"})
            success = False
            error_msg = str(e)
            return

        # ── Step 7: Stream results ────────────────────────────────────────
        # Send results in chunks to avoid massive single payloads
        columns = list(rows[0].keys()) if rows else []
        yield _sse_event("results", {
            "columns": columns,
            "rows": rows,
            "row_count": row_count,
        })

        # ── Step 8: Explanation ───────────────────────────────────────────
        explanation = _build_explanation(question, safe_sql, row_count)
        yield _sse_event("explanation", {"text": explanation})

        # ── Step 9: Save messages to DB ───────────────────────────────────
        await _save_messages(
            db=db,
            conversation_id=conversation_id,
            question=question,
            answer=explanation,
            sql=safe_sql,
            rows=rows,
            columns=columns,
            row_count=row_count,
        )

        yield _sse_event("done", {"message": "Query complete"})

    except Exception as e:
        logger.exception(f"Unexpected pipeline error: {e}")
        success = False
        error_msg = str(e)
        yield _sse_event("error", {"message": f"Internal error: {e}"})

    finally:
        # ── Audit log ─────────────────────────────────────────────────────
        elapsed_ms = (time.monotonic() - start_time) * 1000
        audit = AuditLog(
            user_id=user_id,
            db_connection_id=db_connection_id,
            question=question,
            generated_sql=generated_sql,
            execution_time_ms=elapsed_ms,
            row_count=row_count,
            success=success,
            error_message=error_msg,
        )
        db.add(audit)
        try:
            await db.commit()
        except Exception as e:
            logger.error(f"Failed to save audit log: {e}")


def _build_explanation(question: str, sql: str, row_count: int) -> str:
    """Build a simple human-readable explanation of what was executed."""
    if row_count == 0:
        return f"The query ran successfully but returned no results. This might mean the data doesn't exist or your filters are too specific."
    elif row_count == 1000:
        return f"Found 1,000 rows (limit reached). Consider adding more specific filters to narrow down the results."
    else:
        return f"Found {row_count} row{'s' if row_count != 1 else ''} matching your question."


async def _save_messages(
    db: AsyncSession,
    conversation_id: int,
    question: str,
    answer: str,
    sql: str,
    rows: list,
    columns: list,
    row_count: int,
) -> None:
    """Save the user question and assistant response to the conversation."""
    # User message
    user_msg = Message(
        conversation_id=conversation_id,
        role="user",
        content=question,
    )
    db.add(user_msg)

    # Serialise results so history can re-render the table (cap at 200 rows to keep it lean)
    result_payload = json.dumps({
        "columns": columns,
        "rows": rows[:200],
        "row_count": row_count,
    })

    # Assistant response
    assistant_msg = Message(
        conversation_id=conversation_id,
        role="assistant",
        content=answer,
        generated_sql=sql,
        result_data=result_payload,
    )
    db.add(assistant_msg)

    await db.flush()
