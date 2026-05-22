/**
 * API client for ByteBudd backend.
 * All requests go through /api prefix (proxied by Nginx to backend:8000).
 */

import { OllamaProfile, OllamaProfileCreate, OllamaProfileUpdate } from "@/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost/api";

// Token management
export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("bytebudd_token");
}

export function setToken(token: string): void {
  localStorage.setItem("bytebudd_token", token);
}

export function removeToken(): void {
  localStorage.removeItem("bytebudd_token");
}

// Core fetch wrapper with auth header
async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}/v1${path}`, {
    ...options,
    headers,
  });

  if (response.status === 401) {
    // Don't redirect on the login endpoint itself — let the error bubble up
    if (!path.includes("/auth/login")) {
      removeToken();
      window.location.href = "/login";
      throw new Error("Unauthorized");
    }
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ detail: "Unknown error" }));
    // detail can be a string or an array of validation errors (FastAPI 422)
    const detail = error.detail;
    let message: string;
    if (typeof detail === "string") {
      message = detail;
    } else if (Array.isArray(detail)) {
      message = detail.map((e: { msg?: string }) => e.msg ?? JSON.stringify(e)).join("; ");
    } else {
      message = `HTTP ${response.status}`;
    }
    throw new Error(message);
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// ── Auth API ──────────────────────────────────────────────────────────────

export const authApi = {
  login: (email: string, password: string) =>
    apiFetch<{ access_token: string; token_type: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  me: () => apiFetch<{ id: number; email: string; role: string; is_active: boolean }>("/auth/me"),
};

// ── Database Connection API ───────────────────────────────────────────────

export const dbApi = {
  list: () => apiFetch<unknown[]>("/databases/"),

  create: (data: unknown) =>
    apiFetch<unknown>("/databases/", { method: "POST", body: JSON.stringify(data) }),

  update: (id: number, data: unknown) =>
    apiFetch<unknown>(`/databases/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  delete: (id: number) =>
    apiFetch<void>(`/databases/${id}`, { method: "DELETE" }),

  test: (id: number) =>
    apiFetch<{ success: boolean; message: string; tables_found: number | null }>(
      `/databases/${id}/test`,
      { method: "POST" }
    ),

  getSchema: (id: number) =>
    apiFetch<{ schema: string }>(`/databases/${id}/schema`),
};

// ── Conversations API ─────────────────────────────────────────────────────

export const conversationApi = {
  list: () => apiFetch<unknown[]>("/conversations/"),

  create: (dbConnectionId: number, title?: string) =>
    apiFetch<unknown>("/conversations/", {
      method: "POST",
      body: JSON.stringify({
        db_connection_id: dbConnectionId,
        title: title || "New Conversation",
      }),
    }),

  get: (id: number) => apiFetch<unknown>(`/conversations/${id}`),

  delete: (id: number) =>
    apiFetch<void>(`/conversations/${id}`, { method: "DELETE" }),

  updateTitle: (id: number, title: string) =>
    apiFetch<unknown>(`/conversations/${id}/title?title=${encodeURIComponent(title)}`, {
      method: "PATCH",
    }),
};

// ── Query API (SSE streaming) ─────────────────────────────────────────────

export async function streamQuery(
  question: string,
  conversationId: number,
  dbConnectionId: number,
  onEvent: (event: string, data: unknown) => void,
  onDone: () => void,
  onError: (error: string) => void,
  profileId?: number | null,
  modelName?: string | null,
): Promise<void> {
  const token = getToken();

  const response = await fetch(`${API_BASE}/v1/query/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      question,
      conversation_id: conversationId,
      db_connection_id: dbConnectionId,
      profile_id: profileId ?? null,
      model_name: modelName ?? null,
    }),
  });

  if (!response.ok) {
    const err = await response.json().catch(() => ({ detail: "Stream failed" }));
    onError(err.detail || "Stream failed");
    return;
  }

  const reader = response.body?.getReader();
  if (!reader) {
    onError("No response body");
    return;
  }

  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });

    // Parse SSE format: "event: <name>\ndata: <json>\n\n"
    const lines = buffer.split("\n\n");
    buffer = lines.pop() || "";

    for (const chunk of lines) {
      if (!chunk.trim()) continue;

      const eventMatch = chunk.match(/^event: (\w+)/m);
      const dataMatch = chunk.match(/^data: (.+)/ms);

      if (eventMatch && dataMatch) {
        const eventName = eventMatch[1];
        try {
          const data = JSON.parse(dataMatch[1]);
          onEvent(eventName, data);

          if (eventName === "done") {
            onDone();
          } else if (eventName === "error") {
            onError(data.message || "Unknown error");
          }
        } catch {
          console.warn("Failed to parse SSE data:", dataMatch[1]);
        }
      }
    }
  }
}

// ── Ollama Profile API ────────────────────────────────────────────────────

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
