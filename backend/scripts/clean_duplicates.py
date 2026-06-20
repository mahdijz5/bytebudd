"""Clean up duplicate document records from the database."""
import asyncio
from sqlalchemy import text
from app.core.config import get_settings
from sqlalchemy.ext.asyncio import create_async_engine


async def clean_duplicates():
    settings = get_settings()
    engine = create_async_engine(settings.database_url)

    async with engine.begin() as conn:
        # First, soft delete duplicate records (keep the one with the lowest id per group)
        await conn.execute(text("""
            DELETE FROM documents
            WHERE id NOT IN (
                SELECT MIN(id)
                FROM documents
                WHERE is_deleted = false
                GROUP BY original_filename, user_id
            )
            AND is_deleted = false
        """))

        # Then select all remaining documents
        result = await conn.execute(text(
            'SELECT id, user_id, filename, original_filename, status, is_deleted '
            'FROM documents ORDER BY id'
        ))
        rows = result.fetchall()

    await engine.dispose()

    print("Remaining documents after cleanup:")
    for r in rows:
        print(f"  id={r[0]}, user={r[1]}, filename={r[2]}, orig={r[3]}, status={r[4]}, deleted={r[5]}")

    print(f"\nTotal: {len(rows)} documents")


if __name__ == "__main__":
    asyncio.run(clean_duplicates())