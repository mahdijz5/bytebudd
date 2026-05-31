"""
Microsoft SQL Server connector using pyodbc (sync) wrapped in a thread pool executor
to maintain async compatibility with the FastAPI event loop.
"""

import asyncio
from datetime import date, datetime, time
from decimal import Decimal
from typing import Any, Callable, TypeVar
from uuid import UUID

import pyodbc

from app.db_connectors.base import BaseConnector

T = TypeVar("T")


class SQLServerConnector(BaseConnector):
    def __init__(
        self,
        host: str,
        port: int,
        database: str,
        username: str,
        password: str,
        driver: str,
        instance: str | None = None,
        trust_server_certificate: bool = True,
    ):
        self.host = host
        self.port = port or 1433
        self.database = database
        self.username = username
        self.password = password
        self.driver = driver
        self.instance = instance
        self.trust_server_certificate = trust_server_certificate

    def _build_connection_string(self) -> str:
        """
        Build a valid ODBC connection string from structured parameters.

        SERVER is formatted as host\\instance,port when instance is set,
        otherwise as host,port.
        """
        if self.instance:
            server = f"{self.host}\\{self.instance},{self.port}"
        else:
            server = f"{self.host},{self.port}"

        parts = [
            f"DRIVER={{{self.driver}}}",
            f"SERVER={server}",
            f"DATABASE={self.database}",
            f"UID={self.username}",
            f"PWD={self.password}",
        ]

        if self.trust_server_certificate:
            parts.append("TrustServerCertificate=yes")

        return ";".join(parts)

    def _run_sync(self, fn: Callable[[], T]) -> T:
        """
        Run a blocking pyodbc call in a thread pool so it doesn't block
        the asyncio event loop.
        """
        loop = asyncio.get_running_loop()
        return loop.run_in_executor(None, fn)

    async def test_connection(self) -> dict:
        """
        Test the SQL Server connection by opening a pyodbc connection and querying
        INFORMATION_SCHEMA.TABLES for the count of BASE TABLE entries in the connected database.

        Returns a dict with:
          - success: True/False
          - message: descriptive string
          - tables_found: count of BASE TABLEs (0 on failure)
        """
        conn_str = self._build_connection_string()

        def _connect_and_count():
            conn = pyodbc.connect(conn_str, timeout=10)
            try:
                cursor = conn.cursor()
                cursor.execute(
                    "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES "
                    "WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_CATALOG = ?",
                    self.database,
                )
                row = cursor.fetchone()
                return row[0] if row else 0
            finally:
                conn.close()

        try:
            n = await self._run_sync(_connect_and_count)
            return {
                "success": True,
                "message": f"Connected successfully. Found {n} table(s).",
                "tables_found": n,
            }
        except pyodbc.OperationalError as e:
            return {"success": False, "message": str(e), "tables_found": 0}
        except pyodbc.InterfaceError as e:
            return {"success": False, "message": f"ODBC driver not found: {e}", "tables_found": 0}
        except Exception as e:
            return {"success": False, "message": str(e), "tables_found": 0}

    async def get_schema(self) -> str:
        """
        Introspect the connected SQL Server database and return a formatted schema string.

        Queries INFORMATION_SCHEMA.TABLES for BASE TABLEs and INFORMATION_SCHEMA.COLUMNS
        for each table's columns, then formats the result as:

            TABLE name (
              col TYPE NULLABLE
            )

        Tables are ordered by TABLE_NAME; columns are ordered by ORDINAL_POSITION.
        Exceptions are propagated to the caller.
        """
        conn_str = self._build_connection_string()

        def _fetch_schema():
            conn = pyodbc.connect(conn_str, timeout=10)
            try:
                cursor = conn.cursor()

                cursor.execute(
                    "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES "
                    "WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_CATALOG = ? "
                    "ORDER BY TABLE_NAME",
                    [self.database],
                )
                tables = cursor.fetchall()

                schema_parts = []
                for (table_name,) in tables:
                    cursor.execute(
                        "SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE "
                        "FROM INFORMATION_SCHEMA.COLUMNS "
                        "WHERE TABLE_CATALOG = ? AND TABLE_NAME = ? "
                        "ORDER BY ORDINAL_POSITION",
                        [self.database, table_name],
                    )
                    columns = cursor.fetchall()

                    col_defs = []
                    for (col_name, data_type, is_nullable) in columns:
                        nullable = "NULL" if is_nullable == "YES" else "NOT NULL"
                        col_defs.append(f"  {col_name} {data_type.upper()} {nullable}")

                    schema_parts.append(
                        f"TABLE {table_name} (\n" + ",\n".join(col_defs) + "\n)"
                    )

                return "\n\n".join(schema_parts)
            finally:
                conn.close()

        return await self._run_sync(_fetch_schema)

    def _serialize_value(self, value: Any) -> Any:
        """
        Convert a pyodbc column value to a JSON-serialisable Python type.

        Conversion rules:
          - None          → None
          - datetime/date/time → ISO 8601 string via .isoformat()
          - Decimal       → float
          - bytes         → hex string via .hex()
          - UUID          → str
          - int/float/str/bool → pass through unchanged
          - anything else → str(value)
        """
        if value is None:
            return None
        if isinstance(value, (datetime, date, time)):
            return value.isoformat()
        if isinstance(value, Decimal):
            return float(value)
        if isinstance(value, bytes):
            return value.hex()
        if isinstance(value, UUID):
            return str(value)
        if isinstance(value, (int, float, str, bool)):
            return value
        return str(value)

    async def execute_query(self, sql: str) -> list[dict[str, Any]]:
        """
        Execute a validated SELECT statement via pyodbc and return the results
        as a list of row dicts.

        Each dict maps column name (str) → serialised value.  All values are
        passed through ``_serialize_value`` so the list is always JSON-safe.

        Returns an empty list when the query produces no rows.
        Propagates any connection or execution exception to the caller.
        """
        conn_str = self._build_connection_string()

        def _fetch_rows():
            conn = pyodbc.connect(conn_str, timeout=10)
            try:
                cursor = conn.cursor()
                cursor.execute(sql)
                columns = [desc[0] for desc in cursor.description]
                rows = cursor.fetchall()
                return columns, rows
            finally:
                conn.close()

        columns, rows = await self._run_sync(_fetch_rows)

        return [
            {col: self._serialize_value(val) for col, val in zip(columns, row)}
            for row in rows
        ]
