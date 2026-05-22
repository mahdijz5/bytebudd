"""Add ollama_profiles table

Revision ID: 0003
Revises: 0002
Create Date: 2026-05-22 00:00:00.000000
"""

from typing import Sequence, Union
from alembic import op
import sqlalchemy as sa

revision: str = "0003"
down_revision: Union[str, None] = "0002"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "ollama_profiles",
        sa.Column("id", sa.Integer(), primary_key=True, index=True),
        sa.Column("name", sa.String(100), nullable=False, unique=True),
        sa.Column("host_url", sa.String(500), nullable=False),
        sa.Column("models", sa.JSON(), nullable=False, server_default="[]"),
        sa.Column(
            "is_active",
            sa.Boolean(),
            nullable=False,
            server_default=sa.text("false"),
        ),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
    )
    op.create_index(
        "ix_ollama_profiles_name", "ollama_profiles", ["name"], unique=True
    )


def downgrade() -> None:
    op.drop_index("ix_ollama_profiles_name", table_name="ollama_profiles")
    op.drop_table("ollama_profiles")
