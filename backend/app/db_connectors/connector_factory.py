"""
Factory that creates the correct connector based on db_type.
"""

from app.db_connectors.base import BaseConnector
from app.db_connectors.postgres_connector import PostgresConnector
from app.db_connectors.mysql_connector import MySQLConnector
from app.db_connectors.sqlite_connector import SQLiteConnector
from app.db_connectors.sqlserver_connector import SQLServerConnector
from app.core.encryption import decrypt_password
from app.models.db_connection import DBConnection


def get_connector(conn: DBConnection) -> BaseConnector:
    """
    Given a DBConnection ORM model, create and return the appropriate connector.
    Automatically decrypts the stored password.
    """
    password = ""
    if conn.encrypted_password:
        password = decrypt_password(conn.encrypted_password)

    if conn.db_type == "postgresql":
        return PostgresConnector(
            host=conn.host,
            port=conn.port or 5432,
            database=conn.database_name,
            username=conn.username,
            password=password,
        )

    if conn.db_type in ("mysql", "mariadb"):
        return MySQLConnector(
            host=conn.host,
            port=conn.port or 3306,
            database=conn.database_name,
            username=conn.username,
            password=password,
        )

    if conn.db_type == "sqlite":
        return SQLiteConnector(sqlite_path=conn.sqlite_path or conn.database_name)

    if conn.db_type == "mssql":
        return SQLServerConnector(
            host=conn.host,
            port=conn.port or 1433,
            database=conn.database_name,
            username=conn.username,
            password=password,
            driver=conn.odbc_driver or "ODBC Driver 18 for SQL Server",
            instance=conn.instance_name,
            trust_server_certificate=True,
        )

    raise ValueError(f"Unsupported database type: {conn.db_type}")
