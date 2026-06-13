#!/usr/bin/env python3
"""
Script to generate a bcrypt password hash for manual database insertion.

Usage:
    python backend/scripts/generate_hash.py

Output:
    Prints the bcrypt hash that you can insert into the database manually.
"""

import bcrypt
import sys

# Default credentials - change these if needed
EMAIL = "admin@bytebudd.local"
PASSWORD = "admin123"

def main():
    password = PASSWORD.encode("utf-8")
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password, salt)
    hashed_str = hashed.decode("utf-8")
    
    print("=" * 60)
    print("  Generated Bcrypt Hash")
    print("=" * 60)
    print()
    print(f"  Email:    {EMAIL}")
    print(f"  Password: {PASSWORD}")
    print(f"  Hash:     {hashed_str}")
    print()
    print("=" * 60)
    print()
    print("  SQL to insert manually:")
    print("-" * 60)
    print(f"""
INSERT INTO users (email, password_hash, role, is_active)
VALUES (
    '{EMAIL}',
    '{hashed_str}',
    'admin',
    true
) ON CONFLICT (email) DO UPDATE
SET password_hash = EXCLUDED.password_hash;
""")
    print("-" * 60)
    print()
    print("  Or run inline Python to generate hash:")
    print("-" * 60)
    print('  python -c "import bcrypt; print(bcrypt.hashpw(b\'admin123\', bcrypt.gensalt()).decode())"')
    print("-" * 60)

if __name__ == "__main__":
    main()