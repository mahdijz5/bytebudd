/**
 * Shared TypeScript types for ByteBudd frontend.
 */

export interface User {
  id: number;
  email: string;
  role: "admin" | "user";
  is_active: boolean;
}

export interface DBConnection {
  id: number;
  name: string;
  db_type: "postgresql" | "mysql" | "mariadb" | "sqlite";
  host: string | null;
  port: number | null;
  database_name: string;
  username: string | null;
  is_active: boolean;
}

export interface DBConnectionCreate {
  name: string;
  db_type: "postgresql" | "mysql" | "mariadb" | "sqlite";
  host?: string;
  port?: number;
  database_name: string;
  username?: string;
  password?: string;
  sqlite_path?: string;
}

export interface Conversation {
  id: number;
  title: string;
  db_connection_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: number;
  role: "user" | "assistant";
  content: string;
  generated_sql: string | null;
  result_data: string | null;
  created_at: string;
}

export interface ConversationDetail extends Conversation {
  messages: Message[];
}

// SSE event payloads
export interface SSEThinkingEvent {
  message: string;
}

export interface SSESqlEvent {
  sql: string;
}

export interface SSEResultsEvent {
  columns: string[];
  rows: Record<string, unknown>[];
  row_count: number;
}

export interface SSEExplanationEvent {
  text: string;
}

export interface SSEErrorEvent {
  message: string;
}

export interface SSEDoneEvent {
  message: string;
}

// Chat message display type (client-side only)
export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  sql?: string;
  results?: SSEResultsEvent;
  isStreaming?: boolean;
  error?: string;
}

// ── Ollama Profile types ──────────────────────────────────────────────────

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
