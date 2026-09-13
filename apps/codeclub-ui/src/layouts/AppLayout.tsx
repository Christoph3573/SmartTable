import { Outlet } from "react-router-dom";
import { Sidebar } from "../components/layout/Sidebar";
import { useChatSocket } from "../lib/useChatSocket";

export function AppLayout() {
  useChatSocket();
  return (
    <div className="app-shell flex min-h-screen">
      <Sidebar />
      <main className="app-main flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}
