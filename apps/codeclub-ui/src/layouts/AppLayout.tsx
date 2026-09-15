import { Outlet } from "react-router-dom";
import { Sidebar } from "../components/layout/Sidebar";
import { ThemeToggle } from "../components/ui/ThemeToggle";
import { useChatSocket } from "../lib/useChatSocket";

export function AppLayout() {
  useChatSocket();
  return (
    <div className="app-shell flex min-h-screen">
      <Sidebar />
      <main className="app-main flex-1 overflow-y-auto">
        <Outlet />
      </main>
      {/* Mobil ist die Sidebar-Kopfzeile (mit Toggle) versteckt -> eigener Toggle */}
      <div className="fixed right-3 top-3 z-10 md:hidden">
        <ThemeToggle className="border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-900" />
      </div>
    </div>
  );
}
