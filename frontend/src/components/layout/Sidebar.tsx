"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MessageSquare, Database, Plus, LogOut, Bot, Cpu } from "lucide-react";
import { cn } from "@/lib/utils";
import { logout } from "@/lib/auth";
import { Conversation, User } from "@/types";

interface SidebarProps {
  conversations: Conversation[];
  onNewConversation: () => void;
  user?: User | null;
}

export function Sidebar({ conversations, onNewConversation, user }: SidebarProps) {
  const pathname = usePathname();

  return (
    <aside className="w-64 bg-gray-900 text-white flex flex-col h-full">
      {/* Logo */}
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
            <Bot className="w-5 h-5 text-white" />
          </div>
          <span className="font-bold text-lg">ByteBudd</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="p-3 border-b border-gray-700">
        <NavLink href="/" icon={<MessageSquare className="w-4 h-4" />} active={pathname === "/"}>
          Conversations
        </NavLink>
        <NavLink href="/databases" icon={<Database className="w-4 h-4" />} active={pathname === "/databases"}>
          Databases
        </NavLink>
        {user?.role === "admin" && (
          <NavLink
            href="/admin/ollama-profiles"
            icon={<Cpu className="w-4 h-4" />}
            active={pathname.startsWith("/admin/ollama-profiles")}
          >
            Ollama Profiles
          </NavLink>
        )}
      </nav>

      {/* New conversation button */}
      <div className="p-3">
        <button
          onClick={onNewConversation}
          className="w-full flex items-center gap-2 px-3 py-2 text-sm bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Conversation
        </button>
      </div>

      {/* Conversations list */}
      <div className="flex-1 overflow-y-auto p-2">
        <p className="text-xs font-medium text-gray-500 uppercase tracking-wider px-2 mb-2">
          Recent
        </p>
        {conversations.length === 0 ? (
          <p className="text-gray-500 text-xs px-2">No conversations yet</p>
        ) : (
          conversations.map((conv) => (
            <Link
              key={conv.id}
              href={`/conversations/${conv.id}`}
              className={cn(
                "flex items-center gap-2 px-3 py-2 text-sm rounded-lg mb-1 transition-colors hover:bg-gray-800 truncate",
                pathname === `/conversations/${conv.id}`
                  ? "bg-gray-700 text-white"
                  : "text-gray-400"
              )}
            >
              <MessageSquare className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{conv.title}</span>
            </Link>
          ))
        )}
      </div>

      {/* Logout */}
      <div className="p-3 border-t border-gray-700">
        <button
          onClick={logout}
          className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors"
        >
          <LogOut className="w-4 h-4" />
          Sign out
        </button>
      </div>
    </aside>
  );
}

function NavLink({
  href,
  icon,
  active,
  children,
}: {
  href: string;
  icon: React.ReactNode;
  active: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className={cn(
        "flex items-center gap-2 px-3 py-2 text-sm rounded-lg mb-1 transition-colors",
        active
          ? "bg-gray-700 text-white"
          : "text-gray-400 hover:bg-gray-800 hover:text-white"
      )}
    >
      {icon}
      {children}
    </Link>
  );
}
