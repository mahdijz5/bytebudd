"""Add mssql support: extend db_type_enum and add SQL Server fields

Revision ID: 0004
Revises: 0003
Create Date: 2026-05-22 00:00:00.000000
"""

from typing import Sequence, Union
from alembic import op
import sqlalchemy as sa

revision: str = "0004"
down_revision: Union[str, None] = "0003"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # ALTER TYPE ... ADD VALUE cannot run inside a transaction in PostgreSQL.
    # We use COMMIT to end the current transaction, execute the DDL, then
    # start a new transaction so Alembic can continue normally.
    op.execute("COMMIT")
    op.execute("ALTER TYPE db_type_enum ADD VALUE IF NOT EXISTS 'mssql'")

    # Add SQL Server-specific columns to db_connections
    op.add_column(
        "db_connections",
        sa.Column("instance_name", sa.String(255), nullable=True),
    )
    op.add_column(
        "db_connections",
        sa.Column("odbc_driver", sa.String(255), nullable=True),
    )


def downgrade() -> None:
    # Remove the SQL Server-specific columns first
    op.drop_column("db_connections", "odbc_driver")
    op.drop_column("db_connections", "instance_name")

    # PostgreSQL does not support DROP VALUE from an enum directly.
    # We recreate the enum without 'mssql' and update the column to use it.
    op.execute("COMMIT")

    # Create the replacement enum without 'mssql'
    op.execute(
        "CREATE TYPE db_type_enum_new AS ENUM "
        "('postgresql', 'mysql', 'mariadb', 'sqlite')"
    )

    # Migrate the column to the new enum type
    op.execute(
        "ALTER TABLE db_connections "
        "ALTER COLUMN db_type TYPE db_type_enum_new "
        "USING db_type::text::db_type_enum_new"
    )

    # Drop the old enum and rename the new one
    op.execute("DROP TYPE db_type_enum")
    op.execute("ALTER TYPE db_type_enum_new RENAME TO db_type_enum")
