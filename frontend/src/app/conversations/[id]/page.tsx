"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { isAuthenticated } from "@/lib/auth";
import { conversationApi, dbApi, authApi } from "@/lib/api";
import { Conversation, ConversationDetail, DBConnection, ChatMessage } from "@/types";
import { Sidebar } from "@/components/layout/Sidebar";
import { TopNav } from "@/components/layout/TopNav";
import { ChatWindow } from "@/components/chat/ChatWindow";

export default function ConversationPage() {
  const params = useParams();
  const router = useRouter();
  const convId = Number(params.id);

  const [conversation, setConversation] = useState<ConversationDetail | null>(null);
  const [allConversations, setAllConversations] = useState<Conversation[]>([]);
  const [dbConnection, setDbConnection] = useState<DBConnection | null>(null);
  const [loading, setLoading] = useState(true);
  const [databases, setDatabases] = useState<DBConnection[]>([]);
  const [user, setUser] = useState<import("@/types").User | null>(null);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    if (convId) loadData();
  }, [convId]);

  async function loadData() {
    try {
      const [conv, convs, dbs, me] = await Promise.all([
        conversationApi.get(convId) as Promise<ConversationDetail>,
        conversationApi.list() as Promise<Conversation[]>,
        dbApi.list() as Promise<DBConnection[]>,
        authApi.me(),
      ]);
      setConversation(conv);
      setAllConversations(convs);
      setDatabases(dbs);
      setUser(me as import("@/types").User);

      // Find the connected DB
      if (conv.db_connection_id) {
        const db = dbs.find((d) => d.id === conv.db_connection_id);
        setDbConnection(db || null);
      }
    } catch {
      router.push("/");
    } finally {
      setLoading(false);
    }
  }

  async function handleNewConversation() {
    if (databases.length === 0) {
      router.push("/databases");
      return;
    }
    try {
      const conv = (await conversationApi.create(databases[0].id)) as Conversation;
      router.push(`/conversations/${conv.id}`);
    } catch {
      // handle error
    }
  }

  // Convert DB messages to ChatMessage format
  const initialMessages: ChatMessage[] = (conversation?.messages || []).map(
    (msg) => {
      // Parse stored result_data back into SSEResultsEvent shape
      let results = undefined;
      if (msg.result_data) {
        try {
          results = JSON.parse(msg.result_data);
        } catch {
          // ignore malformed data
        }
      }
      return {
        id: String(msg.id),
        role: msg.role,
        content: msg.content,
        sql: msg.generated_sql || undefined,
        results,
      };
    }
  );

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="w-10 h-10 border-4 border-blue-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!conversation) return null;

  return (
    <div className="flex h-screen">
      <Sidebar
        conversations={allConversations}
        onNewConversation={handleNewConversation}
        user={user}
      />

      <div className="flex-1 flex flex-col min-h-0">
        <TopNav
          title={conversation.title}
          dbConnection={dbConnection}
        />

        {dbConnection ? (
          <div className="flex-1 min-h-0">
            <ChatWindow
              conversationId={convId}
              dbConnectionId={dbConnection.id}
              initialMessages={initialMessages}
            />
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center text-gray-500">
            <p>No database connected to this conversation.</p>
          </div>
        )}
      </div>
    </div>
  );
}
