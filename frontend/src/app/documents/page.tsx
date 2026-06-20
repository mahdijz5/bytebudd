"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { DocumentUpload, DocumentList } from "@/components/upload/DocumentUpload";
import { isAuthenticated } from "@/lib/auth";
import { documentApi, DocumentInfo, DocumentsListResponse } from "@/lib/api";
import { FileText, Upload, Trash2, RefreshCw } from "lucide-react";
import { User } from "@/types";

export default function DocumentsPage() {
  const router = useRouter();
  const [user, setUser] = useState<{ email: string; role: string } | null>(null);
  const [documents, setDocuments] = useState<DocumentInfo[]>([]);
  const [totalDocuments, setTotalDocuments] = useState(0);
  const [loading, setLoading] = useState(true);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Fetch documents from API
  const fetchDocuments = async () => {
    try {
      setLoading(true);
      const response: DocumentsListResponse = await documentApi.list(50, 0);
      setDocuments(response.documents || []);
      setTotalDocuments(response.total || 0);
    } catch (error) {
      console.error("Failed to fetch documents:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    // In production, fetch user from API
    fetchDocuments();
  }, []);

  const handleUploadComplete = async () => {
    await fetchDocuments();
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Are you sure you want to delete this document? This will also remove it from the vector database.")) {
      return;
    }

    try {
      setDeletingId(id);
      await documentApi.delete(id);
      await fetchDocuments();
    } catch (error) {
      console.error("Failed to delete document:", error);
      alert("Failed to delete document. Please try again.");
    } finally {
      setDeletingId(null);
    }
  };

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
        user={user ? { id: 1, email: user.email, role: user.role, is_active: true } as User : null}
      />

      <main className="flex-1 overflow-y-auto">
        <div className="max-w-4xl mx-auto p-8">
          {/* Header */}
          <div className="mb-8">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <FileText className="w-8 h-8 text-blue-600" />
                <h1 className="text-2xl font-bold text-gray-900">Document Management</h1>
              </div>
              <button
                onClick={fetchDocuments}
                className="flex items-center gap-2 px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
              >
                <RefreshCw className="w-4 h-4" />
                Refresh
              </button>
            </div>
            <p className="text-gray-600">
              {totalDocuments} document{totalDocuments !== 1 ? "s" : ""} uploaded · Supported formats: PDF, TXT, DOCX
            </p>
          </div>

          {/* Upload section */}
          <div className="bg-white rounded-xl border border-gray-200 p-6 mb-8">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <Upload className="w-5 h-5 text-blue-600" />
              Upload Document
            </h2>
            <DocumentUpload onUploadComplete={handleUploadComplete} />
          </div>

          {/* Document list */}
          <div className="bg-white rounded-xl border border-gray-200 p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">Uploaded Documents</h2>
              <span className="text-sm text-gray-500">Total: {totalDocuments}</span>
            </div>
            <DocumentList 
              documents={documents.map(doc => ({
                id: doc.id.toString(),
                name: doc.original_filename || doc.filename,
                chunks: doc.chunks_count,
                date: new Date(doc.created_at).toLocaleDateString(),
                status: doc.status,
                fileSize: doc.file_size,
                fileType: doc.file_type,
                errorMessage: doc.error_message,
              }))}
              onDelete={(id: string) => handleDelete(parseInt(id))}
            />
          </div>
        </div>
      </main>
    </div>
  );
}