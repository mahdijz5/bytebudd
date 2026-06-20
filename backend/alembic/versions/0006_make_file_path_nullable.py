"""make file_path nullable

Revision ID: 0006
Revises: 0005
Create Date: 2026-06-13

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision = '0006'
down_revision = '0005'
branch_labels = None
depends_on = None


def upgrade() -> None:
    """Make file_path column nullable."""
    op.alter_column('documents', 'file_path',
                existing_type=sa.VARCHAR(1000),
                nullable=True)


def downgrade() -> None:
    """Revert file_path to not nullable."""
    op.alter_column('documents', 'file_path',
                existing_type=sa.VARCHAR(1000),
                nullable=False)