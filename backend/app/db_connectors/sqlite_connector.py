"""
SQLite connector using aiosqlite.
Supports local SQLite files accessible within the container.
"""

from typing import Any
import aiosqlite

from app.db_connectors.base import BaseConnector


class SQLiteConnector(BaseConnector):
    def __init__(self, sqlite_path: str):
        self.sqlite_path = sqlite_path

    async def test_connection(self) -> dict:
        try:
            async with aiosqlite.connect(self.sqlite_path) as db:
                async with db.execute(
                    "SELECT COUNT(*) FROM sqlite_master WHERE type='table'"
                ) as cursor:
                    row = await cursor.fetchone()
                    table_count = row[0] if row else 0
            return {
                "success": True,
                "message": "Connection successful",
                "tables_found": table_count,
            }
        except Exception as e:
            return {"success": False, "message": str(e), "tables_found": 0}

    async def get_schema(self) -> str:
        schema_parts = []

        async with aiosqlite.connect(self.sqlite_path) as db:
            # Get all tables
            async with db.execute(
                "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
            ) as cursor:
                tables = await cursor.fetchall()

            for (table_name,) in tables:
                # PRAGMA table_info is safe here — table_name comes from
                # sqlite_master (system catalog), not from user input.
                async with db.execute(f"PRAGMA table_info({table_name})") as cursor:
                    columns = await cursor.fetchall()

                col_defs = []
                for col in columns:
                    # PRAGMA table_info: (cid, name, type, notnull, dflt_value, pk)
                    col_name = col[1]
                    col_type = col[2] or "TEXT"
                    not_null = "NOT NULL" if col[3] else "NULL"
                    col_defs.append(f"  {col_name} {col_type.upper()} {not_null}")

                schema_parts.append(
                    f"TABLE {table_name} (\n" + ",\n".join(col_defs) + "\n)"
                )

        return "\n\n".join(schema_parts)

    async def execute_query(self, sql: str) -> list[dict[str, Any]]:
        async with aiosqlite.connect(self.sqlite_path) as db:
            db.row_factory = aiosqlite.Row
            async with db.execute(sql) as cursor:
                rows = await cursor.fetchall()
                return [dict(row) for row in rows]
