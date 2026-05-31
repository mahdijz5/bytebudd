"""
SQL Guard - enforces read-only SQL using sqlglot.

This is the critical security layer that prevents any write operations
from reaching the target database, regardless of what the LLM generates.
"""

import re
import sqlglot
from sqlglot import exp

# Statements that are explicitly allowed
ALLOWED_STATEMENT_TYPES = (
    exp.Select,    # SELECT queries
    exp.With,      # CTEs (WITH ... SELECT)
    exp.Show,      # SHOW TABLES, SHOW DATABASES, etc.
    exp.Command,   # DESCRIBE / EXPLAIN (parsed as Command in some dialects)
)

# Keywords that must never appear
BLOCKED_KEYWORDS = {
    "INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
    "TRUNCATE", "REPLACE", "MERGE", "UPSERT", "GRANT", "REVOKE",
    "EXEC", "EXECUTE", "CALL", "PROCEDURE", "TRIGGER",
}

# Maximum rows returned
DEFAULT_LIMIT = 1000


class SQLGuardError(Exception):
    """Raised when SQL fails validation."""
    pass


def validate_sql(sql: str, dialect: str = "ansi") -> str:
    """
    Validate and sanitize a SQL query.

    Steps:
    1. Strip and clean the input
    2. Check for blocked keywords via regex (fast path)
    3. Parse with sqlglot to verify statement type
    4. Ensure a LIMIT/TOP clause is present (add if missing)

    Args:
        sql: The SQL string to validate.
        dialect: The SQL dialect to use for parsing and output.
                 Use ``"tsql"`` for SQL Server, or ``"ansi"`` (default) for
                 standard SQL. T-SQL output will use TOP N syntax.

    Returns:
        Cleaned, safe SQL string ready for execution.

    Raises:
        SQLGuardError: If the SQL contains disallowed operations.
    """
    if not sql or not sql.strip():
        raise SQLGuardError("Empty SQL query")

    # Step 1: Clean up - strip ANSI escape codes and markdown fences LLMs emit
    sql = _strip_markdown(sql)

    # Step 1b: Detect LLM placeholders like <YourUserID> or <value>
    # These mean the LLM couldn't determine a value and needs more context.
    placeholder = re.search(r"<[^>]+>", sql)
    if placeholder:
        raise SQLGuardError(
            f"The query contains a placeholder '{placeholder.group()}' — "
            "please provide more specific information (e.g. your user ID or name) so the query can be completed."
        )

    # Step 2: Fast keyword check (case-insensitive)
    upper_sql = sql.upper()
    for keyword in BLOCKED_KEYWORDS:
        pattern = r"\b" + keyword + r"\b"
        if re.search(pattern, upper_sql):
            raise SQLGuardError(
                f"Operation '{keyword}' is not allowed. ByteBudd runs read-only queries only."
            )

    # Step 3: Parse with sqlglot using the correct dialect.
    # sqlglot v26+ normalises T-SQL TOP N into a Limit node internally,
    # so we always work with Limit nodes regardless of dialect.
    parse_dialect = dialect if dialect != "ansi" else None
    try:
        statements = sqlglot.parse(sql, dialect=parse_dialect)
    except sqlglot.errors.ParseError as e:
        raise SQLGuardError(f"SQL parse error: {e}")

    if not statements:
        raise SQLGuardError("No valid SQL statement found")

    if len(statements) > 1:
        raise SQLGuardError("Multiple statements are not allowed. Send one query at a time.")

    statement = statements[0]

    # Step 4: Verify it's a read statement
    if not isinstance(statement, ALLOWED_STATEMENT_TYPES):
        stmt_type = type(statement).__name__
        raise SQLGuardError(
            f"Statement type '{stmt_type}' is not allowed. Only SELECT queries are permitted."
        )

    # Step 5: Enforce row cap and return SQL in the correct dialect
    return _ensure_limit(statement, dialect=dialect)


def _strip_markdown(sql: str) -> str:
    """Remove ANSI escape codes and markdown fences that LLMs sometimes emit."""
    # Remove ANSI escape sequences: full form \x1b[...m and orphaned bracket form [...m
    sql = re.sub(r"\x1b\[[0-9;]*m", "", sql)
    sql = re.sub(r"\[[0-9;]*m", "", sql)
    # Remove ```sql ... ``` blocks
    sql = re.sub(r"```(?:sql)?\s*\n?", "", sql, flags=re.IGNORECASE)
    sql = re.sub(r"```\s*$", "", sql, flags=re.MULTILINE)
    return sql.strip().rstrip(";").strip()


def _ensure_limit(statement: exp.Expression, dialect: str = "ansi") -> str:
    """
    Ensure the query has a row cap, then regenerate SQL in the target dialect.

    sqlglot v26+ represents both LIMIT N and TOP N as a Limit node internally.
    Regenerating with dialect="tsql" produces TOP N; anything else produces LIMIT N.

    - No Limit node: inject LIMIT 1000 (becomes TOP 1000 for tsql output).
    - Limit <= 1000: leave as-is.
    - Limit > 1000: cap at 1000.
    """
    out_dialect = dialect if dialect != "ansi" else None

    limit_node = statement.find(exp.Limit)

    if limit_node is None:
        # No row cap — add one
        statement.set("limit", exp.Limit(expression=exp.Literal.number(DEFAULT_LIMIT)))
    else:
        # Row cap exists — enforce ceiling
        try:
            limit_val = int(limit_node.expression.this)
            if limit_val > DEFAULT_LIMIT:
                limit_node.set("expression", exp.Literal.number(DEFAULT_LIMIT))
        except (AttributeError, ValueError):
            pass  # Can't parse the value — leave as-is

    return statement.sql(dialect=out_dialect)
