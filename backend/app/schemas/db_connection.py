"""Database connection Pydantic schemas."""

from typing import Literal
from pydantic import BaseModel, Field


DBType = Literal["postgresql", "mysql", "mariadb", "sqlite", "mssql"]


class DBConnectionCreate(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    db_type: DBType
    host: str | None = None
    port: int | None = None
    database_name: str
    username: str | None = None
    password: str | None = None  # plain-text, encrypted before storage
    sqlite_path: str | None = None
    instance_name: str | None = None  # SQL Server named instance
    odbc_driver: str | None = None    # ODBC driver override


class DBConnectionUpdate(BaseModel):
    name: str | None = None
    host: str | None = None
    port: int | None = None
    database_name: str | None = None
    username: str | None = None
    password: str | None = None
    sqlite_path: str | None = None


class DBConnectionOut(BaseModel):
    id: int
    name: str
    db_type: str
    host: str | None
    port: int | None
    database_name: str
    username: str | None
    is_active: bool
    instance_name: str | None
    odbc_driver: str | None

    model_config = {"from_attributes": True}


class ConnectionTestResult(BaseModel):
    success: bool
    message: str
    tables_found: int | None = None
