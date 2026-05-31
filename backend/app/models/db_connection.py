"""Database connection model - stores user-configured DB connections."""

from datetime import datetime, timezone
from sqlalchemy import String, Integer, DateTime, Enum, ForeignKey, Boolean
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class DBConnection(Base):
    __tablename__ = "db_connections"

    id: Mapped[int] = mapped_column(primary_key=True, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    db_type: Mapped[str] = mapped_column(
        Enum("postgresql", "mysql", "mariadb", "sqlite", "mssql", name="db_type_enum"),
        nullable=False,
    )
    host: Mapped[str | None] = mapped_column(String(255), nullable=True)
    port: Mapped[int | None] = mapped_column(Integer, nullable=True)
    database_name: Mapped[str] = mapped_column(String(255), nullable=False)
    username: Mapped[str | None] = mapped_column(String(255), nullable=True)
    # Password is stored encrypted via Fernet
    encrypted_password: Mapped[str | None] = mapped_column(String(1024), nullable=True)
    # For SQLite: path to the file
    sqlite_path: Mapped[str | None] = mapped_column(String(1024), nullable=True)
    # SQL Server-specific fields
    instance_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    odbc_driver: Mapped[str | None] = mapped_column(String(255), nullable=True)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    # Relationship back to conversations
    conversations: Mapped[list["Conversation"]] = relationship(
        back_populates="db_connection"
    )

    def __repr__(self) -> str:
        return f"<DBConnection id={self.id} name={self.name} type={self.db_type}>"
