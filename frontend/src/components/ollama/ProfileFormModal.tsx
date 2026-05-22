"use client";

import { useState, useEffect } from "react";
import { X, Loader2, RefreshCw, AlertCircle } from "lucide-react";
import { ollamaProfileApi } from "@/lib/api";
import { OllamaProfile } from "@/types";

interface ProfileFormModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: (profile: OllamaProfile) => void;
  initialProfile?: OllamaProfile | null;
}

export function ProfileFormModal({
  open,
  onClose,
  onSaved,
  initialProfile,
}: ProfileFormModalProps) {
  const isEdit = !!initialProfile;

  const [name, setName] = useState("");
  const [hostUrl, setHostUrl] = useState("");
  const [availableModels, setAvailableModels] = useState<string[]>([]);
  const [selectedModels, setSelectedModels] = useState<Set<string>>(new Set());
  const [fetchingModels, setFetchingModels] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [modelsFetched, setModelsFetched] = useState(false);

  // Populate form when editing
  useEffect(() => {
    if (initialProfile) {
      setName(initialProfile.name);
      setHostUrl(initialProfile.host_url);
      setSelectedModels(new Set(initialProfile.models));
      setAvailableModels(initialProfile.models);
      setModelsFetched(true);
    } else {
      setName("");
      setHostUrl("");
      setAvailableModels([]);
      setSelectedModels(new Set());
      setModelsFetched(false);
    }
    setFetchError(null);
    setSaveError(null);
  }, [initialProfile, open]);

  async function handleFetchModels() {
    if (!hostUrl.trim()) return;
    setFetchingModels(true);
    setFetchError(null);
    setAvailableModels([]);
    setSelectedModels(new Set());
    setModelsFetched(false);

    try {
      const result = await ollamaProfileApi.fetchModels(hostUrl.trim());
      setAvailableModels(result.models);
      setModelsFetched(true);
      // Pre-select all models when editing and host didn't change
      if (isEdit && hostUrl.trim() === initialProfile?.host_url) {
        setSelectedModels(new Set(initialProfile.models));
      }
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : "Failed to reach host");
    } finally {
      setFetchingModels(false);
    }
  }

  function toggleModel(model: string) {
    setSelectedModels((prev) => {
      const next = new Set(prev);
      if (next.has(model)) {
        next.delete(model);
      } else {
        next.add(model);
      }
      return next;
    });
  }

  async function handleSave() {
    if (!name.trim() || !hostUrl.trim() || selectedModels.size === 0) return;
    setSaving(true);
    setSaveError(null);

    try {
      let saved: OllamaProfile;
      if (isEdit && initialProfile) {
        saved = await ollamaProfileApi.update(initialProfile.id, {
          name: name.trim(),
          host_url: hostUrl.trim(),
          models: Array.from(selectedModels),
        });
      } else {
        saved = await ollamaProfileApi.create({
          name: name.trim(),
          host_url: hostUrl.trim(),
          models: Array.from(selectedModels),
        });
      }
      onSaved(saved);
      onClose();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Failed to save profile");
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;

  const canSave =
    name.trim().length > 0 &&
    hostUrl.trim().length > 0 &&
    selectedModels.size > 0 &&
    !saving;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      role="dialog"
      aria-modal="true"
      aria-labelledby="profile-modal-title"
    >
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 id="profile-modal-title" className="text-base font-semibold text-gray-900">
            {isEdit ? "Edit Ollama Profile" : "Add Ollama Profile"}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5 space-y-4">
          {/* Profile name */}
          <div>
            <label htmlFor="profile-name" className="block text-sm font-medium text-gray-700 mb-1">
              Profile name
            </label>
            <input
              id="profile-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Local GPU Server"
              maxLength={100}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm text-gray-800 focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none"
            />
          </div>

          {/* Host URL + fetch button */}
          <div>
            <label htmlFor="host-url" className="block text-sm font-medium text-gray-700 mb-1">
              Ollama host URL
            </label>
            <div className="flex gap-2">
              <input
                id="host-url"
                type="url"
                value={hostUrl}
                onChange={(e) => {
                  setHostUrl(e.target.value);
                  setModelsFetched(false);
                  setAvailableModels([]);
                  setSelectedModels(new Set());
                }}
                placeholder="http://192.168.1.99:11434"
                className="flex-1 border border-gray-300 rounded-lg px-3 py-2 text-sm text-gray-800 focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none"
              />
              <button
                type="button"
                onClick={handleFetchModels}
                disabled={!hostUrl.trim() || fetchingModels}
                className="flex items-center gap-1.5 px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 disabled:opacity-50 text-gray-700 rounded-lg transition-colors shrink-0"
              >
                {fetchingModels ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="w-3.5 h-3.5" />
                )}
                {fetchingModels ? "Fetching…" : "Fetch models"}
              </button>
            </div>
            {fetchError && (
              <p className="mt-1.5 text-xs text-red-600 flex items-center gap-1">
                <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                {fetchError}
              </p>
            )}
          </div>

          {/* Model selection */}
          {modelsFetched && (
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">
                Select models{" "}
                <span className="text-gray-400 font-normal">
                  ({selectedModels.size} selected)
                </span>
              </p>
              {availableModels.length === 0 ? (
                <p className="text-sm text-amber-600 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
                  No models available on this host.
                </p>
              ) : (
                <div className="border border-gray-200 rounded-lg divide-y divide-gray-100 max-h-48 overflow-y-auto">
                  {availableModels.map((model) => (
                    <label
                      key={model}
                      className="flex items-center gap-3 px-3 py-2.5 hover:bg-gray-50 cursor-pointer"
                    >
                      <input
                        type="checkbox"
                        checked={selectedModels.has(model)}
                        onChange={() => toggleModel(model)}
                        className="w-4 h-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500"
                      />
                      <span className="text-sm text-gray-700 font-mono">{model}</span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Save error */}
          {saveError && (
            <p className="text-xs text-red-600 flex items-center gap-1">
              <AlertCircle className="w-3.5 h-3.5 shrink-0" />
              {saveError}
            </p>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 px-6 py-4 border-t border-gray-100 bg-gray-50">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-600 hover:text-gray-800 transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={!canSave}
            className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white rounded-lg transition-colors"
          >
            {saving && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            {saving ? "Saving…" : isEdit ? "Save changes" : "Create profile"}
          </button>
        </div>
      </div>
    </div>
  );
}
