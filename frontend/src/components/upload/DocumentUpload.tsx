"use client";

import { useState, useRef, useCallback } from "react";
import { Upload, FileText, X, CheckCircle, AlertCircle } from "lucide-react";
import { uploadRAGFile, UploadResponse } from "@/lib/api";

interface DocumentUploadProps {
  onUploadComplete?: () => void;
}

export function DocumentUpload({ onUploadComplete }: DocumentUploadProps) {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<UploadResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = useCallback(
    async (file: File | null) => {
      if (!file) return;

      // Validate file type
      const validTypes = [
        "application/pdf",
        "text/plain",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      ];
      const validExtensions = [".pdf", ".txt", ".docx"];
      const hasValidExt = validExtensions.some((ext) =>
        file.name.toLowerCase().endsWith(ext)
      );

      if (!validTypes.includes(file.type) && !hasValidExt) {
        setError("Only PDF, TXT, and DOCX files are supported.");
        return;
      }

      setUploading(true);
      setProgress(0);
      setResult(null);
      setError(null);

      try {
        const response = await uploadRAGFile(file, (pct) => setProgress(pct));
        setResult(response);
        onUploadComplete?.();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Upload failed");
      } finally {
        setUploading(false);
      }
    },
    [onUploadComplete]
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const file = fileInputRef.current?.files?.[0];
    handleFileChange(file || null);
  };

  const reset = () => {
    setResult(null);
    setError(null);
    setProgress(0);
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  return (
    <div className="w-full">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="border-2 border-dashed border-gray-300 rounded-xl p-8 text-center hover:border-blue-500 transition-colors">
          <input
            ref={fileInputRef}
            type="file"
            accept=".pdf,.txt,.docx"
            className="hidden"
            disabled={uploading}
          />

          {!uploading && !result && !error && (
            <button
              type="submit"
              disabled={!fileInputRef.current?.files?.length}
              onClick={() => fileInputRef.current?.click()}
              className="flex flex-col items-center gap-3 text-gray-600 disabled:text-gray-400"
            >
              <Upload className="w-10 h-10" />
              <span className="text-sm">
                {fileInputRef.current?.files?.length
                  ? fileInputRef.current.files[0].name
                  : "Click to select a file"}
              </span>
              <span className="text-xs text-gray-400">PDF, TXT, DOCX (max 50MB)</span>
            </button>
          )}

          {uploading && (
            <div className="space-y-3 w-full max-w-xs">
              <div className="flex items-center justify-center gap-2 text-blue-600">
                <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
                <span className="text-sm font-medium">Processing...</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
              <p className="text-xs text-gray-500 text-center">{progress}%</p>
            </div>
          )}

          {result && (
            <div className="space-y-2">
              <div className="flex items-center justify-center gap-2 text-green-600">
                <CheckCircle className="w-6 h-6" />
                <span className="font-medium">{result.filename}</span>
              </div>
              <p className="text-sm text-gray-600">
                {result.chunks_count} chunks created
              </p>
              <p className="text-xs text-gray-500">{result.message}</p>
              <button
                type="button"
                onClick={reset}
                className="mt-2 text-sm text-blue-600 hover:underline"
              >
                Upload another file
              </button>
            </div>
          )}

          {error && (
            <div className="space-y-2">
              <div className="flex items-center justify-center gap-2 text-red-600">
                <AlertCircle className="w-6 h-6" />
                <span className="font-medium">Upload Failed</span>
              </div>
              <p className="text-sm text-gray-600">{error}</p>
              <button
                type="button"
                onClick={reset}
                className="mt-2 text-sm text-blue-600 hover:underline"
              >
                Try again
              </button>
            </div>
          )}
        </div>
      </form>
    </div>
  );
}

export function DocumentList({
  documents,
  onDelete,
}: {
  documents: { id: string; name: string; chunks: number; date: string }[];
  onDelete?: (id: string) => void;
}) {
  if (documents.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        <FileText className="w-10 h-10 mx-auto mb-2 opacity-50" />
        <p className="text-sm">No documents uploaded yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {documents.map((doc) => (
        <div
          key={doc.id}
          className="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
        >
          <div className="flex items-center gap-3">
            <FileText className="w-5 h-5 text-blue-600" />
            <div>
              <p className="font-medium text-sm">{doc.name}</p>
              <p className="text-xs text-gray-500">
                {doc.chunks} chunks · {doc.date}
              </p>
            </div>
          </div>
          {onDelete && (
            <button
              onClick={() => onDelete(doc.id)}
              className="p-1 text-gray-400 hover:text-red-600 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      ))}
    </div>
  );
}