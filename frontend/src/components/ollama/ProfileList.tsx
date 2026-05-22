"use client";

import { useState } from "react";
import { Pencil, Trash2, Loader2, Lock } from "lucide-react";
import { OllamaProfile } from "@/types";

interface ProfileListProps {
  profiles: OllamaProfile[];
  loading: boolean;
  onEdit: (profile: OllamaProfile) => void;
  onDelete: (profile: OllamaProfile) => void;
  onToggleActive: (profile: OllamaProfile) => void;
}

export function ProfileList({
  profiles,
  loading,
  onEdit,
  onDelete,
  onToggleActive,
}: ProfileListProps) {
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16 text-gray-400">
        <Loader2 className="w-6 h-6 animate-spin mr-2" />
        Loading profiles…
      </div>
    );
  }

  if (profiles.length === 0) {
    return (
      <div className="text-center py-16 text-gray-400">
        <p className="text-sm">No profiles configured yet.</p>
        <p className="text-xs mt-1">Click &ldquo;Add Profile&rdquo; to get started.</p>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-xl border border-gray-200">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-gray-50 border-b border-gray-200">
            <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
            <th className="text-left px-4 py-3 font-medium text-gray-600">Host URL</th>
            <th className="text-left px-4 py-3 font-medium text-gray-600">Models</th>
            <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
            <th className="text-right px-4 py-3 font-medium text-gray-600">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {profiles.map((profile) => {
            const isEnvDefault = profile.id === 0;
            const isConfirmingDelete = confirmDeleteId === profile.id;

            return (
              <tr key={profile.id} className="hover:bg-gray-50 transition-colors">
                {/* Name */}
                <td className="px-4 py-3 font-medium text-gray-900">
                  <div className="flex items-center gap-2">
                    {profile.name}
                    {isEnvDefault && (
                      <span className="inline-flex items-center gap-1 text-xs bg-gray-100 text-gray-500 rounded px-1.5 py-0.5">
                        <Lock className="w-3 h-3" />
                        read-only
                      </span>
                    )}
                  </div>
                </td>

                {/* Host URL */}
                <td className="px-4 py-3 text-gray-500 font-mono text-xs max-w-[200px] truncate">
                  {profile.host_url || "—"}
                </td>

                {/* Models */}
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {profile.models.length === 0 ? (
                      <span className="text-gray-400 text-xs">—</span>
                    ) : (
                      profile.models.slice(0, 3).map((m) => (
                        <span
                          key={m}
                          className="inline-block bg-blue-50 text-blue-700 text-xs rounded px-1.5 py-0.5 font-mono"
                        >
                          {m}
                        </span>
                      ))
                    )}
                    {profile.models.length > 3 && (
                      <span className="text-xs text-gray-400">
                        +{profile.models.length - 3} more
                      </span>
                    )}
                  </div>
                </td>

                {/* Status toggle */}
                <td className="px-4 py-3">
                  {isEnvDefault ? (
                    <span className="inline-flex items-center gap-1 text-xs text-green-700 bg-green-50 border border-green-200 rounded-full px-2 py-0.5">
                      <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
                      Active
                    </span>
                  ) : (
                    <button
                      onClick={() => onToggleActive(profile)}
                      className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 ${
                        profile.is_active ? "bg-blue-600" : "bg-gray-200"
                      }`}
                      role="switch"
                      aria-checked={profile.is_active}
                      aria-label={`${profile.is_active ? "Deactivate" : "Activate"} ${profile.name}`}
                    >
                      <span
                        className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                          profile.is_active ? "translate-x-4" : "translate-x-0.5"
                        }`}
                      />
                    </button>
                  )}
                </td>

                {/* Actions */}
                <td className="px-4 py-3 text-right">
                  {isEnvDefault ? (
                    <span className="text-xs text-gray-400">—</span>
                  ) : isConfirmingDelete ? (
                    <div className="flex items-center justify-end gap-2">
                      <span className="text-xs text-gray-500">Delete?</span>
                      <button
                        onClick={() => {
                          setConfirmDeleteId(null);
                          onDelete(profile);
                        }}
                        className="text-xs text-red-600 hover:text-red-700 font-medium"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setConfirmDeleteId(null)}
                        className="text-xs text-gray-500 hover:text-gray-700"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center justify-end gap-1">
                      <button
                        onClick={() => onEdit(profile)}
                        className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                        aria-label={`Edit ${profile.name}`}
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => setConfirmDeleteId(profile.id)}
                        className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                        aria-label={`Delete ${profile.name}`}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
