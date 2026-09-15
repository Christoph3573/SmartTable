import { NavLink, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "../../store/authStore";
import { schoolApi } from "../../api/school";
import { ThemeToggle } from "../ui/ThemeToggle";
import type { Role } from "../../api/auth";

type NavItem = {
  to: string;
  label: string;
  icon: React.ReactNode;
  roles: Role[];
};

const CalendarIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
  </svg>
);

const HomeIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
  </svg>
);

const SwapIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
  </svg>
);

const FolderIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
  </svg>
);

const ChatIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
  </svg>
);

const BookIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 0 0 6 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 0 1 6 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 0 1 6-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0 0 18 18a8.967 8.967 0 0 0-6 2.292m0-14.25v14.25" />
  </svg>
);

const UsersIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
  </svg>
);

const navItems: NavItem[] = [
  { to: "/dashboard", label: "Dashboard", icon: <HomeIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/substitutions", label: "Vertretungsplan", icon: <SwapIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/calendar", label: "Kalender", icon: <CalendarIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/files", label: "Dateien", icon: <FolderIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/homework", label: "Hausaufgaben", icon: <BookIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/chat", label: "Chat", icon: <ChatIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/join", label: "Klasse beitreten", icon: <UsersIcon />, roles: ["student"] },
  { to: "/admin/users", label: "Benutzer", icon: <UsersIcon />, roles: ["superadmin", "admin", "school_admin"] },
  { to: "/admin/classes", label: "Klassen", icon: <UsersIcon />, roles: ["superadmin", "admin", "school_admin", "teacher"] },
  { to: "/admin/schools", label: "Schulen", icon: <UsersIcon />, roles: ["superadmin", "admin"] },
];

export function Sidebar() {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const channels = useQuery({ queryKey: ["channels"], queryFn: schoolApi.channels, enabled: Boolean(user), refetchInterval: 30_000 });
  const unreadTotal = channels.data?.reduce((sum, channel) => sum + (channel.unread_count ?? 0), 0) ?? 0;

  const filteredItems = navItems.filter(
    (item) => user && item.roles.includes(user.role)
  );

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  return (
    <aside className="sidebar sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
      <div className="flex h-20 items-center gap-3 border-b border-gray-200 px-6 dark:border-gray-700">
        <span className="grid size-8 place-items-center rounded-lg bg-indigo-600 text-lg font-bold text-white">S</span>
        <span className="font-bold tracking-tight text-gray-900 dark:text-white">SmartTable</span>
        <span className="ml-auto">
          <ThemeToggle />
        </span>
      </div>

      <nav className="flex-1 overflow-y-auto p-4">
        <ul className="flex flex-col gap-1">
          {filteredItems.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors
                  ${isActive
                    ? "bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300"
                    : "text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-white"
                  }`
                }
              >
                {item.icon}
                <span className="flex-1">{item.label}</span>
                {item.to === "/chat" && unreadTotal > 0 && (
                  <span className="grid min-w-5 place-items-center rounded-full bg-indigo-600 px-1.5 py-0.5 text-[11px] font-semibold text-white">
                    {unreadTotal > 9 ? "9+" : unreadTotal}
                  </span>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="border-t border-gray-200 p-4 dark:border-gray-700">
        <div className="flex items-center gap-3 rounded-lg px-3 py-2">
          <div className="flex size-8 items-center justify-center rounded-full bg-indigo-100 text-sm font-medium text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300">
            {user?.first_name?.[0]}{user?.last_name?.[0]}
          </div>
          <div className="flex-1 overflow-hidden">
            <p className="truncate text-sm font-medium text-gray-900 dark:text-white">
              {user?.first_name} {user?.last_name}
            </p>
            <p className="truncate text-xs text-gray-500 dark:text-gray-400">
              {user?.role === "student" ? "Schüler" : user?.role === "teacher" ? "Lehrer" : user?.role === "school_admin" ? "Schul-Admin" : "Superadmin"}
            </p>
          </div>
          <button
            onClick={handleLogout}
            className="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            title="Abmelden"
          >
            <svg className="size-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9" />
            </svg>
          </button>
        </div>
      </div>
    </aside>
  );
}
