"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { isAuthenticated } from "@/lib/auth";
import { conversationApi, dbApi, authApi } from "@/lib/api";
import { Conversation, DBConnection, User } from "@/types";
import { Sidebar } from "@/components/layout/Sidebar";
import { Database, MessageSquare, Plus, ArrowRight } from "lucide-react";

export default function HomePage() {
  const router = useRouter();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [databases, setDatabases] = useState<DBConnection[]>([]);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    loadData();
  }, []);

  async function loadData() {
    try {
      const [convs, dbs, me] = await Promise.all([
        conversationApi.list() as Promise<Conversation[]>,
        dbApi.list() as Promise<DBConnection[]>,
        authApi.me() as Promise<User>,
      ]);
      setConversations(convs);
      setDatabases(dbs);
      setUser(me);
    } catch {
      // silently handle
    } finally {
      setLoading(false);
    }
  }

  async function handleNewConversation() {
    if (databases.length === 0) {
      router.push("/databases");
      return;
    }
    // Use the first available database for quick start
    setCreating(true);
    try {
      const conv = (await conversationApi.create(databases[0].id)) as Conversation;
      router.push(`/conversations/${conv.id}`);
    } finally {
      setCreating(false);
    }
  }

  if (loading) return <LoadingScreen />;

  return (
    <div className="flex h-screen">
      <Sidebar
        conversations={conversations}
        onNewConversation={handleNewConversation}
        user={user}
      />

      <main className="flex-1 flex flex-col items-center justify-center bg-gray-50 p-8">
        <div className="max-w-2xl w-full text-center">
          <div className="text-6xl mb-6">🤖</div>
          <h1 className="text-3xl font-bold text-gray-900 mb-3">
            Welcome to ByteBudd
          </h1>
          <p className="text-gray-500 mb-10 text-lg">
            Your self-hosted AI SQL assistant. Connect a database and start asking questions.
          </p>

          {/* Stats */}
          <div className="grid grid-cols-2 gap-4 mb-10">
            <div className="bg-white rounded-2xl p-6 border border-gray-200 shadow-sm">
              <div className="flex items-center gap-3 mb-1">
                <div className="w-10 h-10 bg-blue-100 rounded-xl flex items-center justify-center">
                  <MessageSquare className="w-5 h-5 text-blue-600" />
                </div>
                <span className="text-2xl font-bold text-gray-900">{conversations.length}</span>
              </div>
              <p className="text-gray-500 text-sm">Conversations</p>
            </div>
            <div className="bg-white rounded-2xl p-6 border border-gray-200 shadow-sm">
              <div className="flex items-center gap-3 mb-1">
                <div className="w-10 h-10 bg-green-100 rounded-xl flex items-center justify-center">
                  <Database className="w-5 h-5 text-green-600" />
                </div>
                <span className="text-2xl font-bold text-gray-900">{databases.length}</span>
              </div>
              <p className="text-gray-500 text-sm">Databases Connected</p>
            </div>
          </div>

          {/* Actions */}
          <div className="flex gap-4 justify-center">
            {databases.length === 0 ? (
              <button
                onClick={() => router.push("/databases")}
                className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-medium px-6 py-3 rounded-xl transition-colors"
              >
                <Database className="w-4 h-4" />
                Connect a Database
                <ArrowRight className="w-4 h-4" />
              </button>
            ) : (
              <button
                onClick={handleNewConversation}
                disabled={creating}
                className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white font-medium px-6 py-3 rounded-xl transition-colors"
              >
                <Plus className="w-4 h-4" />
                {creating ? "Creating..." : "New Conversation"}
              </button>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

function LoadingScreen() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <div className="w-12 h-12 border-4 border-blue-600 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
        <p className="text-gray-500">Loading ByteBudd...</p>
      </div>
    </div>
  );
}
