#!/usr/bin/env python3
"""
Script to create admin user directly in the database.
Run inside the backend container:
    docker compose exec backend python scripts/insert_admin.py
"""

import asyncio
import os
import sys

# Add app directory to path
sys.path.insert(0, "/app")

import bcrypt
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy import select, text

# ── Config ────────────────────────────────────────────────────────────────────
EMAIL = os.getenv("ADMIN_EMAIL", "admin@bytebudd.local")
PASSWORD = os.getenv("ADMIN_PASSWORD", "admin123")
DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://bytebudd:bytebudd_secret@postgres:5432/bytebudd",
)


def hash_password(password: str) -> str:
    """Hash a password using bcrypt."""
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password.encode("utf-8"), salt)
    return hashed.decode("utf-8")


async def create_admin():
    engine = create_async_engine(DATABASE_URL, echo=False)
    SessionLocal = async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)

    async with SessionLocal() as session:
        # Check if admin already exists
        result = await session.execute(select(User).where(User.email == EMAIL))
        existing = result.scalar_one_or_none()

        if existing:
            print(f"[!] Admin user '{EMAIL}' already exists (id={existing.id})")
            await engine.dispose()
            return

        # Hash password using bcrypt directly
        password_hash = hash_password(PASSWORD)
        print(f"[✓] Password hash generated: {password_hash}")

        # Insert raw SQL to avoid ORM import issues
        await session.execute(
            text(f"""
            INSERT INTO users (email, password_hash, role, is_active)
            VALUES ('{EMAIL}', '{password_hash}', 'admin', true)
            """)
        )
        await session.commit()

        print(f"[✓] Admin user created successfully!")
        print(f"    Email:    {EMAIL}")
        print(f"    Password: {PASSWORD}")
        print(f"    Role:     admin")

    await engine.dispose()


if __name__ == "__main__":
    # Import here so we can use it in the check
    from app.models.user import User
    asyncio.run(create_admin())
