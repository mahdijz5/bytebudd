"use client";

import { useEffect, useRef, useState } from "react";
import { Send, Loader2 } from "lucide-react";
import { ChatMessage, SSEResultsEvent } from "@/types";
import { MessageBubble } from "./MessageBubble";
import { streamQuery } from "@/lib/api";
import { ProfileSelector } from "@/components/ollama/ProfileSelector";

interface ChatWindowProps {
  conversationId: number;
  dbConnectionId: number;
  initialMessages?: ChatMessage[];
}

export function ChatWindow({
  conversationId,
  dbConnectionId,
  initialMessages = [],
}: ChatWindowProps) {
  const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);
  const [input, setInput] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number | null>(null);
  const [selectedModel, setSelectedModel] = useState<string | null>(null);
  const [profileReady, setProfileReady] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function handleSubmit(e?: React.FormEvent) {
    e?.preventDefault();

    const question = input.trim();
    if (!question || isLoading) return;
    // Block if profile selection is required but not made
    if (!profileReady) return;

    setInput("");
    setIsLoading(true);

    // Add user message
    const userMsg: ChatMessage = {
      id: `user-${Date.now()}`,
      role: "user",
      content: question,
    };

    // Add placeholder assistant message
    const assistantId = `assistant-${Date.now()}`;
    const assistantMsg: ChatMessage = {
      id: assistantId,
      role: "assistant",
      content: "",
      isStreaming: true,
    };

    setMessages((prev) => [...prev, userMsg, assistantMsg]);

    // Track SSE state
    let thinkingText = "";
    let sqlText = "";
    let resultsData: SSEResultsEvent | undefined;
    let explanationText = "";

    try {
      await streamQuery(
        question,
        conversationId,
        dbConnectionId,
        // onEvent
        (event: string, data: unknown) => {
          setMessages((prev) =>
            prev.map((msg) => {
              if (msg.id !== assistantId) return msg;

              const typedData = data as Record<string, unknown>;

              if (event === "thinking") {
                thinkingText = String(typedData.message || "");
                return { ...msg, content: thinkingText, isStreaming: true };
              }

              if (event === "sql") {
                sqlText = String(typedData.sql || "");
                return { ...msg, sql: sqlText, isStreaming: true };
              }

              if (event === "results") {
                resultsData = typedData as unknown as SSEResultsEvent;
                return { ...msg, results: resultsData, isStreaming: true };
              }

              if (event === "explanation") {
                explanationText = String(typedData.text || "");
                return {
                  ...msg,
                  content: explanationText,
                  sql: sqlText,
                  results: resultsData,
                  isStreaming: true,
                };
              }

              if (event === "error") {
                return {
                  ...msg,
                  content: "",
                  error: String(typedData.message || "Unknown error"),
                  isStreaming: false,
                };
              }

              return msg;
            })
          );
        },
        // onDone
        () => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === assistantId ? { ...msg, isStreaming: false } : msg
            )
          );
          setIsLoading(false);
        },
        // onError
        (error: string) => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === assistantId
                ? { ...msg, content: "", error, isStreaming: false }
                : msg
            )
          );
          setIsLoading(false);
        },
        // profile selection
        selectedProfileId,
        selectedModel,
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantId
            ? { ...msg, content: "", error: message, isStreaming: false }
            : msg
        )
      );
      setIsLoading(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  }

  return (
    <div className="flex flex-col h-full bg-gray-50">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto py-4">
        {messages.length === 0 ? (
          <EmptyState />
        ) : (
          <>
            {messages.map((msg) => (
              <MessageBubble key={msg.id} message={msg} />
            ))}
            <div ref={bottomRef} />
          </>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-gray-200 bg-white p-4">
        {/* Profile + model selector */}
        <ProfileSelector
          onSelect={(profileId, modelName) => {
            if (profileId === -1) {
              setSelectedProfileId(null);
              setSelectedModel(null);
              setProfileReady(false);
            } else {
              setSelectedProfileId(profileId);
              setSelectedModel(modelName);
              setProfileReady(true);
            }
          }}
          disabled={isLoading}
        />

        <form onSubmit={handleSubmit} className="flex gap-3 items-end">
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={isLoading}
            placeholder="Ask a question about your data... (Enter to send, Shift+Enter for new line)"
            rows={1}
            className="flex-1 resize-none rounded-xl border border-gray-300 px-4 py-3 text-sm text-gray-800 focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition disabled:opacity-50 min-h-[48px] max-h-32"
            style={{ overflowY: "auto" }}
          />
          <button
            type="submit"
            disabled={isLoading || !input.trim() || !profileReady}
            className="w-11 h-11 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-200 text-white rounded-xl flex items-center justify-center transition-colors shrink-0"
          >
            {isLoading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Send className="w-4 h-4" />
            )}
          </button>
        </form>
        <p className="text-xs text-gray-400 mt-2 text-center">
          ByteBudd only runs read-only queries · Max 1000 rows
        </p>
      </div>
    </div>
  );
}

function EmptyState() {
  const examples = [
    "Show me the last 10 orders",
    "How many users registered this month?",
    "What are the top 5 products by revenue?",
    "Show me all orders with status 'pending'",
  ];

  return (
    <div className="flex flex-col items-center justify-center h-full text-center px-8">
      <div className="text-4xl mb-4">🤖</div>
      <h2 className="text-xl font-semibold text-gray-800 mb-2">
        Ask anything about your data
      </h2>
      <p className="text-gray-500 text-sm mb-8 max-w-sm">
        ByteBudd converts your questions into safe, read-only SQL queries using AI.
      </p>
      <div className="grid gap-2 w-full max-w-md">
        {examples.map((example) => (
          <button
            key={example}
            className="text-left text-sm px-4 py-3 bg-white border border-gray-200 rounded-xl hover:border-blue-300 hover:bg-blue-50 transition-colors text-gray-600"
            onClick={() => {
              const textarea = document.querySelector("textarea");
              if (textarea) {
                textarea.value = example;
                textarea.dispatchEvent(new Event("input", { bubbles: true }));
              }
            }}
          >
            &ldquo;{example}&rdquo;
          </button>
        ))}
      </div>
    </div>
  );
}
