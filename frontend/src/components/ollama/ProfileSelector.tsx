"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, Cpu } from "lucide-react";
import { ollamaProfileApi } from "@/lib/api";
import { OllamaProfile } from "@/types";

interface ProfileSelectorProps {
  /** Called whenever the selection changes. profileId=-1 means no valid selection. */
  onSelect: (profileId: number, modelName: string) => void;
  disabled?: boolean;
}

export function ProfileSelector({ onSelect, disabled }: ProfileSelectorProps) {
  const [profiles, setProfiles] = useState<OllamaProfile[]>([]);
  const [selectedProfileId, setSelectedProfileId] = useState<number | null>(null);
  const [selectedModel, setSelectedModel] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadActiveProfiles();
  }, []);

  async function loadActiveProfiles() {
    try {
      const active = await ollamaProfileApi.listActive();
      setProfiles(active);

      // Auto-select when there is exactly one profile with one model
      if (active.length === 1 && active[0].models.length === 1) {
        setSelectedProfileId(active[0].id);
        setSelectedModel(active[0].models[0]);
        onSelect(active[0].id, active[0].models[0]);
      } else if (active.length === 0) {
        onSelect(-1, "");
      }
    } catch {
      setError("Failed to load Ollama profiles.");
      onSelect(-1, "");
    } finally {
      setLoading(false);
    }
  }

  function handleProfileChange(e: React.ChangeEvent<HTMLSelectElement>) {
    const id = Number(e.target.value);
    setSelectedProfileId(id);
    setSelectedModel("");
    onSelect(-1, ""); // reset until model is also chosen
  }

  function handleModelChange(e: React.ChangeEvent<HTMLSelectElement>) {
    const model = e.target.value;
    setSelectedModel(model);
    if (selectedProfileId !== null && model) {
      onSelect(selectedProfileId, model);
    }
  }

  const selectedProfile = profiles.find((p) => p.id === selectedProfileId);

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-xs text-gray-400 mb-2">
        <Cpu className="w-3.5 h-3.5 animate-pulse" />
        Loading AI profiles…
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 mb-2">
        <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
        {error}
      </div>
    );
  }

  if (profiles.length === 0) {
    return (
      <div className="flex items-center gap-2 text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 mb-2">
        <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
        No Ollama profiles are active. Ask your admin to activate one.
      </div>
    );
  }

  // Single profile + single model: auto-selected, show read-only badge
  if (profiles.length === 1 && profiles[0].models.length === 1) {
    return (
      <div className="flex items-center gap-2 text-xs text-gray-500 mb-2">
        <Cpu className="w-3.5 h-3.5 text-blue-500" />
        <span className="bg-blue-50 border border-blue-200 text-blue-700 rounded px-2 py-0.5 font-medium">
          {profiles[0].name} · {profiles[0].models[0]}
        </span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 mb-2 flex-wrap">
      <Cpu className="w-3.5 h-3.5 text-blue-500 shrink-0" />

      {/* Profile selector */}
      <label htmlFor="profile-select" className="sr-only">
        Select Ollama profile
      </label>
      <select
        id="profile-select"
        value={selectedProfileId ?? ""}
        onChange={handleProfileChange}
        disabled={disabled}
        className="text-xs border border-gray-300 rounded-lg px-2 py-1.5 bg-white text-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none disabled:opacity-50"
      >
        <option value="" disabled>
          Select profile…
        </option>
        {profiles.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>

      {/* Model selector — only shown once a profile is picked */}
      {selectedProfile && (
        <>
          <span className="text-gray-400 text-xs">/</span>
          <label htmlFor="model-select" className="sr-only">
            Select model
          </label>
          <select
            id="model-select"
            value={selectedModel}
            onChange={handleModelChange}
            disabled={disabled}
            className="text-xs border border-gray-300 rounded-lg px-2 py-1.5 bg-white text-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none disabled:opacity-50"
          >
            <option value="" disabled>
              Select model…
            </option>
            {selectedProfile.models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </>
      )}
    </div>
  );
}
