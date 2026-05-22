# Implementation Tasks: Ollama Profile Management

## Task 1: Database migration — add `ollama_profiles` table

- [ ] Create `backend/alembic/versions/0003_add_ollama_profiles.py`
  - `upgrade()`: create `ollama_profiles` table with all columns (`id`, `name`, `host_url`, `models` JSON, `is_active`, `created_at`, `updated_at`) and unique index on `name`
  - `downgrade()`: drop index and table

**Requirements covered:** 3.4

---

## Task 2: Backend — SQLAlchemy model

- [ ] Create `backend/app/models/ollama_profile.py`
  - `OllamaProfile` class extending `Base`
  - All columns matching the migration
  - `updated_at` with `onupdate=func.now()`
- [ ] Register the model in `backend/app/models/__init__.py` so Alembic picks it up

**Requirements covered:** 3.4

---

## Task 3: Backend — Pydantic schemas

- [ ] Create `backend/app/schemas/ollama_profile.py`
  - `OllamaProfileCreate` — `name` (1–100 chars), `host_url` (HttpUrl), `models` (list, min 1 item)
  - `OllamaProfileUpdate` — all fields optional
  - `OllamaProfileOut` — full profile with `from_attributes=True`
  - `FetchModelsRequest` — `host_url: HttpUrl`
  - `FetchModelsResponse` — `models: list[str]`
  - `ActivateRequest` — `is_active: bool`

**Requirements covered:** 2.5, 3.1–3.3, 5.1

---

## Task 4: Backend — service layer

- [ ] Create `backend/app/services/ollama_profile_service.py`
  - Custom exceptions: `ProfileNotFoundError`, `ProfileNameConflictError`, `HostUnreachableError`, `InvalidHostResponseError`
  - `fetch_models_from_host(host_url: str) -> list[str]` — calls `/api/tags` with 10s timeout; raises appropriate exceptions on timeout, non-200, or malformed JSON
  - `create_profile(db, data: OllamaProfileCreate) -> OllamaProfile` — checks name uniqueness (409), persists with `is_active=False`
  - `list_profiles(db) -> list[OllamaProfile]` — ordered by `created_at DESC`
  - `get_profile(db, id) -> OllamaProfile` — raises `ProfileNotFoundError` if missing
  - `update_profile(db, id, data: OllamaProfileUpdate) -> OllamaProfile` — partial update; if `host_url` changed, re-fetches models before saving; checks name uniqueness on rename; sets `updated_at`
  - `delete_profile(db, id) -> None` — raises `ProfileNotFoundError` if missing
  - `set_active(db, id, is_active: bool) -> OllamaProfile` — flips flag, sets `updated_at`; raises `ProfileNotFoundError` if missing
  - `get_active_profiles(db) -> list[OllamaProfile]` — returns all where `is_active=True`
  - `resolve_ollama_config(db, profile_id, model_name) -> tuple[str, str]` — returns `(host_url, model)` from DB profile, or falls back to `settings.ollama_base_url` / `settings.ollama_model`

**Requirements covered:** 2.1–2.5, 3.1–3.5, 4.1–4.4, 5.1–5.5, 6.1–6.3, 7.1–7.4, 9.1–9.4, 10.1–10.4

---

## Task 5: Backend — API router

- [ ] Create `backend/app/api/v1/ollama_profiles.py`
  - `POST /fetch-models` (admin) — calls `fetch_models_from_host`, returns `FetchModelsResponse`
  - `GET /` (admin) — calls `list_profiles`; includes synthetic "Environment Default" entry when `OLLAMA_BASE_URL` is set
  - `POST /` (admin) — calls `create_profile`; returns 201
  - `GET /active` (any authenticated user) — calls `get_active_profiles`
  - `GET /{id}` (admin) — calls `get_profile`
  - `PATCH /{id}` (admin) — calls `update_profile`
  - `DELETE /{id}` (admin) — calls `delete_profile`; returns 204
  - `PATCH /{id}/active` (admin) — calls `set_active`
  - Map custom exceptions to correct HTTP status codes (404, 409, 422, 502)
- [ ] Register router in `backend/app/api/v1/__init__.py` with prefix `/ollama-profiles`

**Requirements covered:** 1.3–1.4, 2.1–2.5, 3.1–3.5, 4.1–4.4, 5.1–5.5, 6.1–6.3, 7.1–7.4, 9.1–9.4, 10.3

---

## Task 6: Backend — update `OllamaClient` and `query_pipeline`

- [ ] In `backend/app/llm/ollama_client.py`:
  - Add standalone `async def fetch_models(host_url: str) -> list[str]` function (used by service layer)
  - Add `async def generate_with_profile(host_url: str, model: str, prompt: str) -> str` function
- [ ] In `backend/app/schemas/conversation.py`:
  - Add optional `profile_id: int | None = None` and `model_name: str | None = None` to `QueryRequest`
- [ ] In `backend/app/services/query_pipeline.py`:
  - Accept `profile_id` and `model_name` parameters in `run_query_pipeline`
  - If both provided, call `resolve_ollama_config` and use `generate_with_profile`
  - Otherwise fall back to existing `ollama_client.generate()`
- [ ] In `backend/app/api/v1/query.py`:
  - Pass `profile_id` and `model_name` from `QueryRequest` to `run_query_pipeline`

**Requirements covered:** 8.4, 10.1–10.2

---

## Task 7: Frontend — TypeScript types

- [ ] In `frontend/src/types/index.ts`:
  - Add `OllamaProfile` interface
  - Add `OllamaProfileCreate` interface
  - Add `OllamaProfileUpdate` interface

**Requirements covered:** (supports all frontend tasks)

---

## Task 8: Frontend — API client methods

- [ ] In `frontend/src/lib/api.ts`:
  - Add `ollamaProfileApi` object with: `fetchModels`, `list`, `create`, `update`, `delete`, `setActive`, `listActive`
  - Update `streamQuery` to accept optional `profileId` and `modelName` parameters and include them in the request body

**Requirements covered:** 8.4

---

## Task 9: Frontend — Sidebar update

- [ ] In `frontend/src/components/layout/Sidebar.tsx`:
  - Add `user` prop of type `User | null`
  - Conditionally render "Ollama Profiles" nav link (with `Cpu` icon from lucide-react) only when `user?.role === "admin"`
  - Link points to `/admin/ollama-profiles`
- [ ] Update all parent components that render `Sidebar` to pass the `user` prop

**Requirements covered:** 1.1–1.2

---

## Task 10: Frontend — ProfileFormModal component

- [ ] Create `frontend/src/components/ollama/ProfileFormModal.tsx`
  - Props: `open`, `onClose`, `onSaved`, `initialProfile?: OllamaProfile`
  - Two-step flow:
    - Step 1: Name input + Host URL input + "Fetch Models" button
    - On fetch: calls `ollamaProfileApi.fetchModels(host_url)`, shows loading state, then renders checkbox list of models
    - If fetch fails: shows inline error message
    - If empty model list: shows "No models available on this host" message
    - Step 2: At least one model must be checked to enable Save
  - On save: calls `create` or `update` depending on whether `initialProfile` is provided
  - Shows validation errors inline

**Requirements covered:** 2.1–2.5, 3.1–3.3, 5.1–5.2, 9.1–9.3

---

## Task 11: Frontend — ProfileList component

- [ ] Create `frontend/src/components/ollama/ProfileList.tsx`
  - Props: `profiles: OllamaProfile[]`, `onEdit`, `onDelete`, `onToggleActive`, `loading`
  - Renders a table with columns: Name, Host URL, Models (count badge), Status (toggle switch), Actions (Edit button, Delete button)
  - Active toggle calls `onToggleActive(profile.id, !profile.is_active)`
  - Delete button shows a confirmation before calling `onDelete`
  - "Environment Default" row (id=0) renders without Edit/Delete/Toggle actions

**Requirements covered:** 4.1–4.4, 6.1–6.3, 7.1–7.3

---

## Task 12: Frontend — Admin Ollama Profiles page

- [ ] Create `frontend/src/app/admin/ollama-profiles/page.tsx`
  - Client component with auth guard (redirect to `/login` if not authenticated, redirect to `/` if not admin)
  - On mount: fetch all profiles via `ollamaProfileApi.list()`
  - Renders page header with "Add Profile" button
  - Renders `ProfileList` with loaded profiles
  - "Add Profile" opens `ProfileFormModal` in create mode
  - Edit action opens `ProfileFormModal` in edit mode with the selected profile
  - Delete action calls `ollamaProfileApi.delete(id)` then refreshes list
  - Toggle active calls `ollamaProfileApi.setActive(id, !current)` then refreshes list
  - Shows loading skeleton while fetching
  - Shows error state if fetch fails

**Requirements covered:** 1.1, 3.1–3.5, 4.1–4.4, 5.1–5.5, 6.1–6.3, 7.1–7.4

---

## Task 13: Frontend — ProfileSelector component

- [ ] Create `frontend/src/components/ollama/ProfileSelector.tsx`
  - Props: `onSelect: (profileId: number, modelName: string) => void`, `disabled?: boolean`
  - On mount: fetches active profiles via `ollamaProfileApi.listActive()`
  - If 0 active profiles: renders warning banner "No Ollama profiles are active. Ask your admin to activate one." and calls `onSelect` with `(-1, "")` to signal disabled state
  - If 1 active profile with 1 model: auto-selects silently, calls `onSelect` immediately
  - Otherwise: renders profile dropdown → model dropdown (model dropdown populates when profile is selected)
  - Accessible: proper `<label>` elements, keyboard navigable

**Requirements covered:** 8.1–8.6

---

## Task 14: Frontend — integrate ProfileSelector into query interface

- [ ] Locate the query submission form in the conversations page
- [ ] Add `ProfileSelector` above the question input
- [ ] Track selected `profileId` and `modelName` in component state
- [ ] Pass them to `streamQuery` when submitting
- [ ] Disable the submit button when no profile/model is selected (unless env fallback is active)

**Requirements covered:** 8.1–8.6
