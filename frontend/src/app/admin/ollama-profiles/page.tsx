"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, Cpu } from "lucide-react";
import { isAuthenticated } from "@/lib/auth";
import { authApi, ollamaProfileApi, conversationApi } from "@/lib/api";
import { OllamaProfile, User, Conversation } from "@/types";
import { Sidebar } from "@/components/layout/Sidebar";
import { ProfileList } from "@/components/ollama/ProfileList";
import { ProfileFormModal } from "@/components/ollama/ProfileFormModal";

export default function OllamaProfilesPage() {
  const router = useRouter();

  const [user, setUser] = useState<User | null>(null);
  const [profiles, setProfiles] = useState<OllamaProfile[]>([]);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<OllamaProfile | null>(null);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    loadData();
  }, []);

  async function loadData() {
    setLoading(true);
    setError(null);
    try {
      const [me, profileList, convs] = await Promise.all([
        authApi.me() as Promise<User>,
        ollamaProfileApi.list(),
        conversationApi.list() as Promise<Conversation[]>,
      ]);

      // Redirect non-admins
      if (me.role !== "admin") {
        router.push("/");
        return;
      }

      setUser(me);
      setProfiles(profileList);
      setConversations(convs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load data");
    } finally {
      setLoading(false);
    }
  }

  function handleOpenCreate() {
    setEditingProfile(null);
    setModalOpen(true);
  }

  function handleOpenEdit(profile: OllamaProfile) {
    setEditingProfile(profile);
    setModalOpen(true);
  }

  async function handleDelete(profile: OllamaProfile) {
    try {
      await ollamaProfileApi.delete(profile.id);
      setProfiles((prev) => prev.filter((p) => p.id !== profile.id));
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete profile");
    }
  }

  async function handleToggleActive(profile: OllamaProfile) {
    try {
      const updated = await ollamaProfileApi.setActive(profile.id, !profile.is_active);
      setProfiles((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to update profile");
    }
  }

  function handleSaved(saved: OllamaProfile) {
    setProfiles((prev) => {
      const exists = prev.find((p) => p.id === saved.id);
      if (exists) {
        return prev.map((p) => (p.id === saved.id ? saved : p));
      }
      // New profile: insert at top (before env default at end)
      const envDefault = prev.find((p) => p.id === 0);
      const rest = prev.filter((p) => p.id !== 0);
      return envDefault ? [saved, ...rest, envDefault] : [saved, ...rest];
    });
  }

  async function handleNewConversation() {
    router.push("/");
  }

  return (
    <div className="flex h-screen">
      <Sidebar
        conversations={conversations}
        onNewConversation={handleNewConversation}
        user={user}
      />

      <main className="flex-1 overflow-y-auto bg-gray-50">
        <div className="max-w-5xl mx-auto px-6 py-8">
          {/* Page header */}
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-blue-100 rounded-xl flex items-center justify-center">
                <Cpu className="w-5 h-5 text-blue-600" />
              </div>
              <div>
                <h1 className="text-xl font-bold text-gray-900">Ollama Profiles</h1>
                <p className="text-sm text-gray-500">
                  Manage AI model configurations for your users
                </p>
              </div>
            </div>
            <button
              onClick={handleOpenCreate}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-xl transition-colors"
            >
              <Plus className="w-4 h-4" />
              Add Profile
            </button>
          </div>

          {/* Error state */}
          {error && (
            <div className="mb-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-xl px-4 py-3">
              {error}
            </div>
          )}

          {/* Profile list */}
          <ProfileList
            profiles={profiles}
            loading={loading}
            onEdit={handleOpenEdit}
            onDelete={handleDelete}
            onToggleActive={handleToggleActive}
          />

          {/* Info note */}
          {!loading && profiles.length > 0 && (
            <p className="mt-4 text-xs text-gray-400 text-center">
              Active profiles are available to all users in the query interface.
              Multiple profiles can be active simultaneously.
            </p>
          )}
        </div>
      </main>

      {/* Create / Edit modal */}
      <ProfileFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={handleSaved}
        initialProfile={editingProfile}
      />
    </div>
  );
}
