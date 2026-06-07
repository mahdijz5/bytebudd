"use client";

import { useState, useRef, useEffect } from "react";
import { Send, Bot, User, FileText } from "lucide-react";
import { streamRAGQuery, SourceInfo } from "@/lib/api";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  sources?: SourceInfo[];
  isSQL?: boolean;
}

export function RAGChat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = async () => {
    const question = input.trim();
    if (!question || streaming) return;

    // Add user message
    const userMsg: Message = {
      id: Date.now().toString(),
      role: "user",
      content: question,
    };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setStreaming(true);
    setError(null);

    // Add placeholder assistant message
    const assistantMsgId = (Date.now() + 1).toString();
    setMessages((prev) => [
      ...prev,
      {
        id: assistantMsgId,
        role: "assistant",
        content: "",
      },
    ]);

    let currentText = "";

    const onEvent = (event: string, data: unknown) => {
      switch (event) {
        case "thinking":
          currentText = (data as { message?: string }).message || "Thinking...";
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantMsgId ? { ...m, content: currentText } : m
            )
          );
          break;

        case "is_sql":
          currentText = (data as { message?: string }).message || "Routing to SQL...";
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantMsgId
                ? { ...m, content: currentText, isSQL: true }
                : m
            )
          );
          break;

        case "answer": {
          const answerData = data as { answer?: string; sources?: SourceInfo[] };
          currentText = answerData.answer || "";
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantMsgId
                ? {
                    ...m,
                    content: currentText,
                    sources: answerData.sources,
                  }
                : m
            )
          );
          break;
        }

        case "done":
          setStreaming(false);
          break;

        case "error":
          setError((data as { message?: string }).message || "Unknown error");
          setStreaming(false);
          break;
      }
    };

    const onDone = () => {
      setStreaming(false);
    };

    const onError = (err: string) => {
      setError(err);
      setStreaming(false);
    };

    await streamRAGQuery(question, onEvent, onDone, onError);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center text-gray-500">
            <Bot className="w-16 h-16 mb-4 opacity-30" />
            <h2 className="text-xl font-semibold mb-2">RAG Document Chat</h2>
            <p className="text-sm max-w-md">
              Ask questions about your uploaded documents. The AI will search through your documents
              to provide accurate answers with source citations.
            </p>
          </div>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex gap-3 ${
              msg.role === "user" ? "justify-end" : "justify-start"
            }`}
          >
            {msg.role === "assistant" && (
              <div className="w-8 h-8 bg-blue-600 rounded-full flex items-center justify-center shrink-0">
                <Bot className="w-4 h-4 text-white" />
              </div>
            )}

            <div
              className={`max-w-[70%] rounded-xl p-3 ${
                msg.role === "user"
                  ? "bg-blue-600 text-white"
                  : "bg-gray-100 text-gray-900"
              }`}
            >
              {msg.isSQL && (
                <div className="text-xs opacity-75 mb-1">
                  ⚠️ This question requires a database query. Please switch to SQL mode.
                </div>
              )}
              <p className="text-sm whitespace-pre-wrap">{msg.content}</p>

              {/* Sources */}
              {msg.sources && msg.sources.length > 0 && (
                <div className="mt-2 pt-2 border-t border-gray-200">
                  <p className="text-xs font-medium mb-1">Sources:</p>
                  {msg.sources.map((source, i) => (
                    <div key={i} className="text-xs text-gray-600 mb-1">
                      <FileText className="w-3 h-3 inline mr-1" />
                      {source.filename} (score: {source.score.toFixed(2)})
                    </div>
                  ))}
                </div>
              )}
            </div>

            {msg.role === "user" && (
              <div className="w-8 h-8 bg-gray-600 rounded-full flex items-center justify-center shrink-0">
                <User className="w-4 h-4 text-white" />
              </div>
            )}
          </div>
        ))}

        {error && (
          <div className="text-center text-red-600 text-sm bg-red-50 p-2 rounded-lg">
            {error}
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="border-t p-4">
        <div className="flex gap-2">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question about your documents..."
            disabled={streaming}
            className="flex-1 px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
          />
          <button
            onClick={handleSend}
            disabled={streaming || !input.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Send className="w-5 h-5" />
          </button>
        </div>
      </div>
    </div>
  );
}