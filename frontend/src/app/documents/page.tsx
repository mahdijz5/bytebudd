"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { DocumentUpload, DocumentList } from "@/components/upload/DocumentUpload";
import { isAuthenticated } from "@/lib/auth";
import { FileText, Upload } from "lucide-react";
import { User } from "@/types";

interface DocumentInfo {
  id: string;
  name: string;
  chunks: number;
  date: string;
}

export default function DocumentsPage() {
  const router = useRouter();
  const [user, setUser] = useState<{ email: string; role: string } | null>(null);
  const [documents, setDocuments] = useState<DocumentInfo[]>([
    // Mock data - will be replaced with API call
    { id: "1", name: "company_handbook.pdf", chunks: 45, date: "2024-01-15" },
    { id: "2", name: "product_specs.docx", chunks: 23, date: "2024-01-14" },
  ]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push("/login");
      return;
    }
    // In production, fetch user from API
    setLoading(false);
  }, []);

  const handleUploadComplete = () => {
    // In production, refresh document list from API
  };

  const handleDelete = (id: string) => {
    setDocuments((prev) => prev.filter((d) => d.id !== id));
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
            <div className="flex items-center gap-3 mb-2">
              <FileText className="w-8 h-8 text-blue-600" />
              <h1 className="text-2xl font-bold text-gray-900">Document Management</h1>
            </div>
            <p className="text-gray-600">
              Upload documents for the AI to search through. Supported formats: PDF, TXT, DOCX
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
            <h2 className="text-lg font-semibold mb-4">Uploaded Documents</h2>
            <DocumentList documents={documents} onDelete={handleDelete} />
          </div>
        </div>
      </main>
    </div>
  );
}