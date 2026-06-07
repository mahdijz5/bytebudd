"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { RAGChat } from "@/components/chat/RAGChat";
import { isAuthenticated } from "@/lib/auth";
import { User } from "@/types";

export default function RAGChatPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    setLoading(false);
  }, []);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-12 h-12 border-4 border-blue-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="flex h-screen">
      <Sidebar
        conversations={[]}
        onNewConversation={() => router.push("/")}
        user={user}
      />

      <main className="flex-1 flex flex-col bg-gray-50">
        <RAGChat />
      </main>
    </div>
  );
}