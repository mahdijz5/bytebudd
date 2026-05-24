# ByteBudd

A self-hosted AI SQL assistant. Connect your databases, ask questions in plain English, and get safe read-only SQL results — powered by a local Ollama model.

---

## What it does

- Converts natural language questions into SQL using a local LLM (via Ollama)
- Executes queries against your connected databases (PostgreSQL, MySQL, MariaDB, SQLite)
- Streams results back in real time using Server-Sent Events (SSE)
- Enforces read-only access — SQL is validated at the AST level using `sqlglot`; no INSERT, UPDATE, DELETE, DROP, or any write operation ever runs
- Auto-detects WordPress/WooCommerce and PrestaShop schemas and injects platform-specific prompt hints
- Stores full conversation history per user, including generated SQL and result data
- Supports multiple Ollama hosts via admin-managed profiles
- Logs every query attempt with timing, row count, and error details in an audit log

---

## Tech stack

| Layer      | Technology                                                        |
|------------|-------------------------------------------------------------------|
| Frontend   | Next.js 15, React 18, TypeScript, Tailwind CSS, Radix UI         |
| Backend    | FastAPI (Python 3.12), SQLAlchemy 2.0 async, Uvicorn             |
| App DB     | PostgreSQL 16 (internal)                                          |
| Migrations | Alembic                                                           |
| AI         | Ollama HTTP API (default model: `qwen2.5-coder:8b`)              |
| SQL Safety | `sqlglot` AST-based validation + regex keyword blocking           |
| Auth       | JWT HS256 via `python-jose`, bcrypt via `passlib`                 |
| Encryption | Fernet (AES-128-CBC) via `cryptography` for stored DB passwords   |
| Proxy      | Nginx 1.25                                                        |
| Containers | Docker Compose (dev + prod)                                       |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Compose v2
- [Ollama](https://ollama.com) running and accessible (same machine or remote host)
- The model pulled in Ollama:

```bash
ollama pull qwen2.5-coder:8b
```

---

## Setup

### 1. Clone and configure

```bash
git clone <repo-url>
cd bytebudd
cp .env.example .env
```

Open `.env` and set at minimum:

```env
SECRET_KEY=<a long random string>
ENCRYPTION_KEY=<exactly 32 characters>
OLLAMA_BASE_URL=http://<your-ollama-host>:11434
```

### 2. Start the stack

```bash
./start.sh
```

On first run, add `--init` to run migrations and create the admin user:

```bash
./start.sh --init
```

The app will be available at **http://localhost**.  
API docs (Swagger UI) are at **http://localhost/api/docs**.

Default admin credentials (change after first login):
- Email: `admin@bytebudd.local`
- Password: `admin123`

---

## Running in production

```bash
./start.sh --prod --build --init
```

This uses `docker-compose.prod.yml` overrides: no volume mounts, `restart: always`, JSON log rotation, and 2 Uvicorn workers. Alembic migrations run automatically on startup.

Make sure your `.env` has strong values for `SECRET_KEY` and `ENCRYPTION_KEY` before deploying.

---

## Stopping

```bash
./stop.sh              # stop containers, keep data
./stop.sh --clean      # stop + delete all database volumes
./stop.sh --purge      # --clean + remove built images
```

---

## Creating additional users

Only admins can create new users. Use the `/api/v1/auth/register` endpoint (requires a valid admin JWT) or the Swagger UI at `/api/docs`.

To create the first admin manually:

```bash
docker compose exec backend python scripts/create_admin.py
```

Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` in `.env` to override the defaults.

---

## Ollama profiles

By default, ByteBudd uses the `OLLAMA_BASE_URL` and `OLLAMA_MODEL` from your `.env`. Admins can additionally configure named Ollama profiles pointing to different hosts and models via the admin panel. Users can then select a profile (and a specific model from that profile) when running a query.

The environment default always appears as a synthetic read-only entry in the profiles list.

---

## API reference

All endpoints are under `/api/v1/`. Interactive docs at `/api/docs`.

| Group             | Endpoint                          | Auth     | Description                              |
|-------------------|-----------------------------------|----------|------------------------------------------|
| Auth              | `POST /auth/login`                | —        | Get a JWT token                          |
| Auth              | `POST /auth/register`             | Admin    | Create a new user                        |
| Auth              | `GET /auth/me`                    | User     | Current user profile                     |
| Auth              | `GET /auth/users`                 | Admin    | List all users                           |
| Databases         | `GET /databases/`                 | User     | List your DB connections                 |
| Databases         | `POST /databases/`                | User     | Add a DB connection                      |
| Databases         | `PUT /databases/{id}`             | User     | Update a DB connection                   |
| Databases         | `DELETE /databases/{id}`          | User     | Remove a DB connection                   |
| Databases         | `POST /databases/{id}/test`       | User     | Test connectivity                        |
| Databases         | `GET /databases/{id}/schema`      | User     | Fetch schema as text                     |
| Conversations     | `GET /conversations/`             | User     | List conversations                       |
| Conversations     | `POST /conversations/`            | User     | Start a new conversation                 |
| Conversations     | `GET /conversations/{id}`         | User     | Get conversation with messages           |
| Conversations     | `DELETE /conversations/{id}`      | User     | Delete conversation                      |
| Conversations     | `PATCH /conversations/{id}/title` | User     | Rename conversation                      |
| Query             | `POST /query/stream`              | User     | Ask a question (SSE stream)              |
| Query             | `GET /query/ollama/status`        | User     | Check Ollama availability                |
| Query             | `POST /query/ollama/pull`         | User     | Pull/download the Ollama model           |
| Ollama Profiles   | `GET /ollama-profiles/active`     | User     | List active profiles                     |
| Ollama Profiles   | `GET /ollama-profiles/`           | Admin    | List all profiles                        |
| Ollama Profiles   | `POST /ollama-profiles/`          | Admin    | Create a profile                         |
| Ollama Profiles   | `PATCH /ollama-profiles/{id}`     | Admin    | Update a profile                         |
| Ollama Profiles   | `DELETE /ollama-profiles/{id}`    | Admin    | Delete a profile                         |
| Ollama Profiles   | `PATCH /ollama-profiles/{id}/active` | Admin | Activate / deactivate a profile       |
| Ollama Profiles   | `POST /ollama-profiles/fetch-models` | Admin | Probe a host and list its models      |
| Health            | `GET /api/health`                 | —        | Liveness check                           |

### SSE event types (from `POST /query/stream`)

| Event         | Payload                                      | Description                        |
|---------------|----------------------------------------------|------------------------------------|
| `thinking`    | `{ message: string }`                        | Pipeline progress update           |
| `sql`         | `{ sql: string }`                            | Validated SQL about to be executed |
| `results`     | `{ columns, rows, row_count }`               | Query results (up to 1000 rows)    |
| `explanation` | `{ text: string }`                           | Human-readable summary             |
| `done`        | `{ message: string }`                        | Pipeline complete                  |
| `error`       | `{ message: string }`                        | Something went wrong               |

---

## Security model

- **JWT auth**: HS256 tokens, 60-minute expiry by default
- **Password hashing**: bcrypt
- **Stored credential encryption**: Fernet (AES-128-CBC + HMAC-SHA256); the `ENCRYPTION_KEY` is SHA-256 hashed to produce a valid 32-byte key
- **SQL Guard** (the critical layer):
  1. Strips markdown code fences LLMs sometimes add
  2. Regex keyword blocklist (INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, TRUNCATE, MERGE, GRANT, REVOKE, EXEC, CALL, …)
  3. `sqlglot` AST parse — only `SELECT`, `WITH…SELECT` (CTEs), `SHOW`, and `DESCRIBE` are allowed
  4. Rejects multi-statement queries
  5. Auto-injects `LIMIT 1000` if missing; caps any higher limit at 1000
- **Ownership checks**: every user-scoped resource (connections, conversations) is filtered by `user_id`

---

## Project structure

```
bytebudd/
├── backend/
│   ├── app/
│   │   ├── api/v1/           # Route handlers: auth, databases, conversations, query, ollama_profiles
│   │   ├── core/             # Config, DB engine, security (JWT/bcrypt), encryption (Fernet), DI deps
│   │   ├── db_connectors/    # Async connectors: asyncpg (PG), aiomysql (MySQL/MariaDB), aiosqlite (SQLite)
│   │   ├── llm/              # Ollama HTTP client (singleton + profile-aware helpers)
│   │   ├── models/           # SQLAlchemy ORM models (user, db_connection, conversation, message, audit_log, ollama_profile)
│   │   ├── prompts/          # SQL prompt builder with WordPress/WooCommerce + PrestaShop hints
│   │   ├── schemas/          # Pydantic request/response schemas
│   │   └── services/         # query_pipeline, sql_guard, ollama_profile_service
│   ├── alembic/              # Database migrations (3 versions)
│   ├── scripts/
│   │   └── create_admin.py   # Bootstrap admin user
│   └── requirements.txt
├── frontend/
│   └── src/
│       ├── app/              # Pages: login, home, databases, conversations, admin
│       ├── components/       # Chat UI, layout, Ollama profile selector
│       ├── lib/              # Typed API client, auth helpers, utilities
│       └── types/            # Shared TypeScript interfaces
├── infrastructure/
│   ├── nginx/                # Reverse proxy config
│   └── mysql/                # Sample MySQL init script (for local testing)
├── docker-compose.yml        # Development stack
├── docker-compose.prod.yml   # Production overrides
├── start.sh                  # Start script (--init, --prod, --build flags)
├── stop.sh                   # Stop script (--clean, --purge flags)
└── .env.example              # Environment variable template
```

---

## Environment variables

| Variable                      | Description                                        | Default                          |
|-------------------------------|----------------------------------------------------|----------------------------------|
| `SECRET_KEY`                  | JWT signing secret — **change in production**      | insecure default                 |
| `ENCRYPTION_KEY`              | Fernet key for stored DB passwords                 | insecure default                 |
| `DATABASE_URL`                | Internal PostgreSQL connection string              | points to `postgres` container   |
| `OLLAMA_BASE_URL`             | Ollama API base URL                                | `http://ollama:11434`            |
| `OLLAMA_MODEL`                | Default model name                                 | `qwen2.5-coder:8b`               |
| `OLLAMA_TIMEOUT`              | Ollama request timeout in seconds                  | `120`                            |
| `CORS_ORIGINS`                | Comma-separated list of allowed origins            | `http://localhost`               |
| `NEXT_PUBLIC_API_URL`         | API base URL seen by the browser                   | `http://localhost/api`           |
| `ACCESS_TOKEN_EXPIRE_MINUTES` | JWT expiry in minutes                              | `60`                             |
| `ENVIRONMENT`                 | `development` or `production`                      | `development`                    |
| `ADMIN_EMAIL`                 | Default admin email for `create_admin.py`          | `admin@bytebudd.local`           |
| `ADMIN_PASSWORD`              | Default admin password for `create_admin.py`       | `admin123`                       |
