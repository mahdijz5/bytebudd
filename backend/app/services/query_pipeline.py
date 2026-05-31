"""
Core query pipeline: question → schema → LLM → validate → execute → stream.

Each step yields SSE events so the frontend can show live progress.
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

# Maps db_type values to sqlglot dialect names
_DIALECT_MAP = {
    "postgresql": "postgresql",
    "mysql": "mysql",
    "mariadb": "mysql",
    "sqlite": "sqlite",
    "mssql": "tsql",
}


def _sse(event: str, data: dict) -> str:
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
    Run the full query pipeline and yield SSE events.

    Events yielded:
      - thinking: progress updates while working
      - sql: the validated SQL query
      - results: query results (columns + rows)
      - explanation: plain-language summary
      - done: pipeline finished successfully
      - error: something went wrong
    """
    start_time = time.monotonic()
    generated_sql: str | None = None
    row_count: int | None = None
    success = True
    error_msg: str | None = None

    try:
        # Step 1: Load the database connection record
        result = await db.execute(
            select(DBConnection).where(
                DBConnection.id == db_connection_id,
                DBConnection.user_id == user_id,  # Ensure the user owns this connection
            )
        )
        db_conn = result.scalar_one_or_none()
        if not db_conn:
            yield _sse("error", {"message": "Database connection not found"})
            return

        connector = get_connector(db_conn)
        dialect = _DIALECT_MAP.get(db_conn.db_type, "ansi")

        # Step 2: Fetch the database schema
        yield _sse("thinking", {"message": "Reading database schema..."})
        try:
            schema = await connector.get_schema()
        except Exception as e:
            logger.error("Schema fetch failed: %s", e)
            yield _sse("error", {"message": f"Failed to read schema: {e}"})
            return

        if not schema.strip():
            yield _sse("error", {"message": "Database schema is empty or unreadable"})
            return

        logger.debug("Schema fetched: %d chars", len(schema))

        # Step 3: Build the LLM prompt
        yield _sse("thinking", {"message": "Analyzing your question..."})
        prompt = build_sql_prompt(
            question=question,
            schema=schema,
            dialect=dialect,
            current_user_id=user_id,
        )

        # Step 4: Call the LLM
        yield _sse("thinking", {"message": "Generating SQL with AI..."})
        try:
            if profile_id is not None and model_name is not None:
                host_url, resolved_model = await resolve_ollama_config(db, profile_id, model_name)
                raw_sql = await generate_with_profile(host_url, resolved_model, prompt)
            else:
                raw_sql = await ollama_client.generate(prompt)
        except OllamaError as e:
            logger.error("LLM error: %s", e)
            yield _sse("error", {"message": f"AI model error: {e}"})
            return

        if raw_sql.upper().startswith("ERROR:"):
            yield _sse("error", {"message": raw_sql})
            return

        # Step 5: Validate and sanitize the SQL
        logger.debug("LLM raw output: %r", raw_sql)
        try:
            safe_sql = validate_sql(raw_sql, dialect=dialect)
            logger.debug("SQL after validation (dialect=%s): %s", dialect, safe_sql)
        except SQLGuardError as e:
            logger.warning("SQL validation failed. Raw: %r | Error: %s", raw_sql, e)
            yield _sse("error", {"message": f"SQL validation failed: {e}"})
            success = False
            error_msg = str(e)
            return

        generated_sql = safe_sql
        yield _sse("sql", {"sql": safe_sql})

        # Step 6: Execute the query
        yield _sse("thinking", {"message": "Executing query..."})
        try:
            rows = await connector.execute_query(safe_sql)
            row_count = len(rows)
        except Exception as e:
            logger.error("Query execution failed: %s", e)
            yield _sse("error", {"message": f"Query execution failed: {e}"})
            success = False
            error_msg = str(e)
            return

        # Step 7: Send results
        columns = list(rows[0].keys()) if rows else []
        yield _sse("results", {"columns": columns, "rows": rows, "row_count": row_count})

        # Step 8: Send plain-language explanation
        explanation = _build_explanation(row_count)
        yield _sse("explanation", {"text": explanation})

        # Step 9: Persist messages to conversation history
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

        yield _sse("done", {"message": "Query complete"})

    except Exception as e:
        logger.exception("Unexpected pipeline error: %s", e)
        success = False
        error_msg = str(e)
        yield _sse("error", {"message": f"Internal error: {e}"})

    finally:
        # Always write an audit log entry, even on failure
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
            logger.error("Failed to save audit log: %s", e)


def _build_explanation(row_count: int) -> str:
    """Return a plain-language summary of the query result."""
    if row_count == 0:
        return "The query ran successfully but returned no results. Your filters may be too specific."
    if row_count >= 1000:
        return "Found 1,000 rows (limit reached). Add more specific filters to narrow down the results."
    return f"Found {row_count} row{'s' if row_count != 1 else ''}."


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
    """Save the user question and assistant response to conversation history."""
    db.add(Message(conversation_id=conversation_id, role="user", content=question))

    # Cap stored rows at 200 to keep the payload lean
    result_payload = json.dumps({
        "columns": columns,
        "rows": rows[:200],
        "row_count": row_count,
    })

    db.add(Message(
        conversation_id=conversation_id,
        role="assistant",
        content=answer,
        generated_sql=sql,
        result_data=result_payload,
    ))

    await db.flush()
