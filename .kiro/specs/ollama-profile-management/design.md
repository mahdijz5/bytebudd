# Design Document: Ollama Profile Management

## Overview

This document describes the technical design for the Ollama Profile Management feature. The feature adds a database-backed system for managing multiple named Ollama profiles (host URL + selected models), replacing the current single-host environment-variable approach. Admins manage profiles via a dedicated UI page; regular users choose an active profile and model before submitting queries.

The implementation touches four layers:
1. **Database** — new `ollama_profiles` table + migration
2. **Backend** — new SQLAlchemy model, Pydantic schemas, service layer, and API router
3. **Frontend** — new admin page, updated sidebar, updated query interface
4. **Integration** — updated `query_pipeline.py` to route through the selected profile

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (Next.js / React)                                  │
│                                                              │
│  Sidebar ──► /admin/ollama-profiles  (admin only)           │
│                  OllamaProfilesPage                          │
│                  ├── ProfileList (table of all profiles)     │
│                  ├── AddProfileModal (host URL → fetch       │
│                  │   models → select → save)                 │
│                  └── EditProfileModal                        │
│                                                              │
│  QueryInterface ──► ProfileSelector (active profiles only)  │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST / SSE
┌──────────────────────────▼──────────────────────────────────┐
│  Backend (FastAPI)                                           │
│                                                              │
│  /api/v1/ollama-profiles/*   (admin-only CRUD + activate)   │
│  /api/v1/ollama-profiles/fetch-models  (POST, admin)        │
│  /api/v1/ollama-profiles/active        (GET, all users)     │
│  /api/v1/query/stream  ──► query_pipeline (profile-aware)   │
└──────────────────────────┬──────────────────────────────────┘
                           │ SQLAlchemy async
┌──────────────────────────▼──────────────────────────────────┐
│  PostgreSQL                                                  │
│  ollama_profiles table                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Design

### New table: `ollama_profiles`

| Column | Type | Notes |
|---|---|---|
| `id` | `INTEGER` PK | auto-increment |
| `name` | `VARCHAR(100)` | unique, not null |
| `host_url` | `VARCHAR(500)` | not null, e.g. `http://192.168.1.99:11434` |
| `models` | `JSON` | list of model name strings, not null |
| `is_active` | `BOOLEAN` | default `false`, not null |
| `created_at` | `TIMESTAMPTZ` | server default `now()` |
| `updated_at` | `TIMESTAMPTZ` | server default `now()`, updated on every write |

`models` is stored as a JSON array of strings (e.g. `["llama3:8b", "qwen2.5-coder:8b"]`). PostgreSQL's native JSON type is used; SQLAlchemy's `JSON` column type handles serialisation.

### Migration

A new Alembic migration `0003_add_ollama_profiles.py` creates the table and adds a unique index on `name`.

---

## Backend Design

### File structure (new/modified files)

```
backend/app/
├── models/
│   └── ollama_profile.py          # NEW — SQLAlchemy model
├── schemas/
│   └── ollama_profile.py          # NEW — Pydantic schemas
├── services/
│   └── ollama_profile_service.py  # NEW — business logic
├── api/v1/
│   ├── __init__.py                # MODIFIED — register new router
│   └── ollama_profiles.py         # NEW — FastAPI router
├── llm/
│   └── ollama_client.py           # MODIFIED — add fetch_models(), profile-aware generate()
└── services/
    └── query_pipeline.py          # MODIFIED — accept profile_id + model_name
```

### SQLAlchemy Model (`models/ollama_profile.py`)

```python
class OllamaProfile(Base):
    __tablename__ = "ollama_profiles"

    id: Mapped[int] = mapped_column(primary_key=True, index=True)
    name: Mapped[str] = mapped_column(String(100), unique=True, nullable=False)
    host_url: Mapped[str] = mapped_column(String(500), nullable=False)
    models: Mapped[list[str]] = mapped_column(JSON, nullable=False, default=list)
    is_active: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())
```

### Pydantic Schemas (`schemas/ollama_profile.py`)

```python
class OllamaProfileCreate(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    host_url: HttpUrl
    models: list[str] = Field(..., min_length=1)  # at least one model

class OllamaProfileUpdate(BaseModel):
    name: str | None = Field(None, min_length=1, max_length=100)
    host_url: HttpUrl | None = None
    models: list[str] | None = None

class OllamaProfileOut(BaseModel):
    id: int
    name: str
    host_url: str
    models: list[str]
    is_active: bool
    created_at: datetime
    updated_at: datetime
    model_config = ConfigDict(from_attributes=True)

class FetchModelsRequest(BaseModel):
    host_url: HttpUrl

class FetchModelsResponse(BaseModel):
    models: list[str]

class ActivateRequest(BaseModel):
    is_active: bool
```

### Service Layer (`services/ollama_profile_service.py`)

All database operations are isolated here. The router delegates to this service.

Key functions:

| Function | Description |
|---|---|
| `fetch_models_from_host(host_url)` | Calls `{host_url}/api/tags` with 10s timeout; returns `list[str]`; raises `HostUnreachableError`, `InvalidHostResponseError` |
| `create_profile(db, data)` | Validates uniqueness, persists, returns `OllamaProfileOut` |
| `list_profiles(db)` | Returns all profiles ordered by `created_at DESC` |
| `get_profile(db, id)` | Returns single profile or raises 404 |
| `update_profile(db, id, data)` | Partial update; re-fetches models if `host_url` changed |
| `delete_profile(db, id)` | Hard delete; returns nothing |
| `set_active(db, id, is_active)` | Flips `is_active`; multiple profiles can be active simultaneously |
| `get_active_profiles(db)` | Returns all profiles where `is_active=True` |
| `resolve_ollama_config(db, profile_id, model_name)` | Returns `(host_url, model_name)` for query routing; falls back to env vars if no active profiles |

### API Router (`api/v1/ollama_profiles.py`)

All endpoints except `GET /active` require `get_admin_user` dependency.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/ollama-profiles/fetch-models` | admin | Probe a host URL and return its models |
| `GET` | `/ollama-profiles/` | admin | List all profiles |
| `POST` | `/ollama-profiles/` | admin | Create a profile |
| `GET` | `/ollama-profiles/{id}` | admin | Get a single profile |
| `PATCH` | `/ollama-profiles/{id}` | admin | Partial update |
| `DELETE` | `/ollama-profiles/{id}` | admin | Delete |
| `PATCH` | `/ollama-profiles/{id}/active` | admin | Activate / deactivate |
| `GET` | `/ollama-profiles/active` | any authenticated user | List active profiles (for Profile_Selector) |

### Updated `OllamaClient` (`llm/ollama_client.py`)

Add a standalone `fetch_models(host_url: str) -> list[str]` function (not a method on the singleton) so the service layer can call it without instantiating a client tied to a specific host.

Add `generate_with_profile(host_url, model, prompt) -> str` that accepts explicit host/model rather than reading from `settings`.

The existing `ollama_client` singleton and `generate()` method remain unchanged for backward compatibility.

### Updated `query_pipeline.py`

`run_query_pipeline` gains two optional parameters: `profile_id: int | None` and `model_name: str | None`.

- If both are provided, call `resolve_ollama_config(db, profile_id, model_name)` to get `(host_url, model)` and use `generate_with_profile`.
- If neither is provided (legacy path), fall back to the existing `ollama_client.generate()`.

The `QueryRequest` schema gains optional `profile_id` and `model_name` fields.

### Updated `QueryRequest` schema

```python
class QueryRequest(BaseModel):
    question: str
    conversation_id: int
    db_connection_id: int
    profile_id: int | None = None   # NEW
    model_name: str | None = None   # NEW
```

### Error handling

Custom exceptions in the service layer map to HTTP responses in the router:

| Exception | HTTP Status |
|---|---|
| `ProfileNotFoundError` | 404 |
| `ProfileNameConflictError` | 409 |
| `HostUnreachableError` | 502 |
| `InvalidHostResponseError` | 502 |
| `HostValidationError` | 422 |

---

## Frontend Design

### New files

```
frontend/src/
├── app/
│   └── admin/
│       └── ollama-profiles/
│           └── page.tsx           # NEW — admin Ollama Profiles page
├── components/
│   └── ollama/
│       ├── ProfileList.tsx        # NEW — table of profiles with actions
│       ├── ProfileFormModal.tsx   # NEW — create/edit modal
│       └── ProfileSelector.tsx    # NEW — user-facing profile+model picker
└── lib/
    └── api.ts                     # MODIFIED — add ollamaProfileApi
└── types/
    └── index.ts                   # MODIFIED — add OllamaProfile types
```

### Modified files

- `Sidebar.tsx` — add "Ollama Profiles" nav link, visible only when `user.role === "admin"`
- `types/index.ts` — add `OllamaProfile`, `OllamaProfileCreate`, `OllamaProfileUpdate` interfaces
- `api.ts` — add `ollamaProfileApi` object
- Query page — wrap the submit form with `ProfileSelector`

### New TypeScript types

```typescript
export interface OllamaProfile {
  id: number;
  name: string;
  host_url: string;
  models: string[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface OllamaProfileCreate {
  name: string;
  host_url: string;
  models: string[];
}

export interface OllamaProfileUpdate {
  name?: string;
  host_url?: string;
  models?: string[];
}
```

### New API client methods (`api.ts`)

```typescript
export const ollamaProfileApi = {
  fetchModels: (host_url: string) =>
    apiFetch<{ models: string[] }>("/ollama-profiles/fetch-models", {
      method: "POST",
      body: JSON.stringify({ host_url }),
    }),

  list: () => apiFetch<OllamaProfile[]>("/ollama-profiles/"),

  create: (data: OllamaProfileCreate) =>
    apiFetch<OllamaProfile>("/ollama-profiles/", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  update: (id: number, data: OllamaProfileUpdate) =>
    apiFetch<OllamaProfile>(`/ollama-profiles/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),

  delete: (id: number) =>
    apiFetch<void>(`/ollama-profiles/${id}`, { method: "DELETE" }),

  setActive: (id: number, is_active: boolean) =>
    apiFetch<OllamaProfile>(`/ollama-profiles/${id}/active`, {
      method: "PATCH",
      body: JSON.stringify({ is_active }),
    }),

  listActive: () => apiFetch<OllamaProfile[]>("/ollama-profiles/active"),
};
```

### Admin page (`app/admin/ollama-profiles/page.tsx`)

Client component. On mount, fetches all profiles via `ollamaProfileApi.list()`. Renders:

1. **Header** — "Ollama Profiles" title + "Add Profile" button
2. **ProfileList** — table with columns: Name, Host URL, Models (badge count), Status (active/inactive toggle), Actions (Edit, Delete)
3. **ProfileFormModal** — shared create/edit modal:
   - Step 1: Name field + Host URL field + "Fetch Models" button
   - Step 2 (after fetch): Checkbox list of available models
   - Save button (disabled until ≥1 model selected)

### ProfileSelector component

Used inside the query interface. Props: `onSelect: (profileId: number, modelName: string) => void`.

- Fetches active profiles from `ollamaProfileApi.listActive()` on mount
- If 0 active profiles: shows warning banner, disables submit
- If 1 active profile with 1 model: auto-selects silently
- Otherwise: renders a two-level select (profile → model)

### Sidebar update

```tsx
// Only render when user.role === "admin"
{user?.role === "admin" && (
  <NavLink href="/admin/ollama-profiles" icon={<Cpu className="w-4 h-4" />} active={...}>
    Ollama Profiles
  </NavLink>
)}
```

The `Sidebar` component needs access to the current user. The existing `page.tsx` already fetches the user via `authApi.me()`. The user object will be passed down as a prop to `Sidebar`.

---

## Data Flow

### Admin creates a profile

```
Admin fills host URL
  → POST /ollama-profiles/fetch-models { host_url }
  → Backend calls {host_url}/api/tags (10s timeout)
  → Returns { models: ["llama3:8b", ...] }
  → Admin selects models, enters name
  → POST /ollama-profiles/ { name, host_url, models }
  → Backend persists, returns OllamaProfile (is_active: false)
  → Frontend adds row to table
```

### Admin activates a profile

```
Admin clicks toggle in ProfileList
  → PATCH /ollama-profiles/{id}/active { is_active: true }
  → Backend flips flag, returns updated profile
  → Frontend updates row in table
```

### User submits a query

```
User opens query interface
  → GET /ollama-profiles/active
  → ProfileSelector renders active profiles
  → User selects profile + model
  → POST /query/stream { question, conversation_id, db_connection_id, profile_id, model_name }
  → query_pipeline calls resolve_ollama_config(db, profile_id, model_name)
  → generate_with_profile(host_url, model, prompt)
  → SSE stream back to frontend
```

### Fallback (no active profiles)

```
POST /query/stream { ..., profile_id: null, model_name: null }
  → resolve_ollama_config returns (settings.ollama_base_url, settings.ollama_model)
  → existing ollama_client.generate() path
```

---

## Migration

New file: `backend/alembic/versions/0003_add_ollama_profiles.py`

```python
revision = "0003"
down_revision = "0002"

def upgrade():
    op.create_table(
        "ollama_profiles",
        sa.Column("id", sa.Integer(), primary_key=True, index=True),
        sa.Column("name", sa.String(100), nullable=False, unique=True),
        sa.Column("host_url", sa.String(500), nullable=False),
        sa.Column("models", sa.JSON(), nullable=False, server_default="[]"),
        sa.Column("is_active", sa.Boolean(), nullable=False, server_default="false"),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.text("now()")),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.text("now()")),
    )
    op.create_index("ix_ollama_profiles_name", "ollama_profiles", ["name"], unique=True)

def downgrade():
    op.drop_index("ix_ollama_profiles_name", "ollama_profiles")
    op.drop_table("ollama_profiles")
```

---

## Backward Compatibility

The existing `ollama_client` singleton and `generate()` method are untouched. The `query_pipeline` falls back to them when `profile_id` is `None`. This means:

- Existing deployments with only env-var config continue to work without any database profiles.
- The `GET /ollama-profiles/active` endpoint returns an empty list when no profiles exist, and the `ProfileSelector` shows a fallback message but does not block the query if the env-var fallback is available.

The "Environment Default" read-only entry (Requirement 10.3) is synthesised in the `GET /ollama-profiles/` response by the service layer — it is not stored in the database. It appears as a special entry with `id: 0` and `is_active: true` when `OLLAMA_BASE_URL` is set.
