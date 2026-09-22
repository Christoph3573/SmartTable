import { NavLink, useNavigate } from "react-router-dom";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "../../store/authStore";
import { useSettingsStore } from "../../store/settingsStore";
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

const LogoutIcon = () => (
  <svg className="size-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9" />
  </svg>
);

const JoinIcon = () => (
  <svg className="size-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
  </svg>
);

const ChevronUpIcon = () => (
  <svg className="size-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 15.75 7.5-7.5 7.5 7.5" />
  </svg>
);

const GearIcon = () => (
  <svg className="size-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 0 1 0 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 0 1 0-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
    <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
  </svg>
);

const CardsIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M6 6.878V6a2.25 2.25 0 0 1 2.25-2.25h7.5A2.25 2.25 0 0 1 18 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 0 0 4.5 9v.878m13.5-3A2.25 2.25 0 0 1 19.5 9v.878m0 0a2.246 2.246 0 0 0-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0 1 21 12v6a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 18v-6c0-.98.626-1.813 1.5-2.122" />
  </svg>
);

const SparkIcon = () => (
  <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456Z" />
  </svg>
);

type NavSection = {
  title: string;
  items: NavItem[];
};

const mainNavItems: NavItem[] = [
  { to: "/timetable", label: "Stundenplan", icon: <CalendarIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/substitutions", label: "Vertretungsplan", icon: <SwapIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/calendar", label: "Kalender", icon: <CalendarIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/files", label: "Dateien", icon: <FolderIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/homework", label: "Hausaufgaben", icon: <BookIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/chat", label: "Chat", icon: <ChatIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
];

const learnNavItems: NavItem[] = [
  { to: "/lernen/lernplan", label: "Lernplan", icon: <BookIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/lernen/vokabeln", label: "Vokabeln", icon: <CardsIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
  { to: "/lernen/ki-chat", label: "KI-Chat", icon: <SparkIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
];

const dashboardNavItems: NavItem[] = [
  { to: "/dashboard", label: "Dashboard", icon: <HomeIcon />, roles: ["student", "teacher", "school_admin", "superadmin", "admin"] },
];

const adminNavItems: NavItem[] = [
  { to: "/admin/users", label: "Benutzer", icon: <UsersIcon />, roles: ["superadmin", "admin", "school_admin"] },
  { to: "/admin/classes", label: "Klassen", icon: <UsersIcon />, roles: ["superadmin", "admin", "school_admin", "teacher"] },
  { to: "/admin/schools", label: "Schulen", icon: <UsersIcon />, roles: ["superadmin", "admin"] },
];

export function Sidebar() {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const provider = useSettingsStore((s) => s.provider);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);
  const channels = useQuery({ queryKey: ["channels"], queryFn: schoolApi.channels, enabled: Boolean(user), refetchInterval: 30_000 });
  const unreadTotal = channels.data?.reduce((sum, channel) => sum + (channel.unread_count ?? 0), 0) ?? 0;

  const visible = (items: NavItem[]) =>
    items.filter((item) => user && item.roles.includes(user.role));

  const dashboardItems = visible(dashboardNavItems);
  const schoolItems = visible(mainNavItems);
  const learnItems = visible(learnNavItems);
  const adminItems = visible(adminNavItems);
  const canJoin = user?.role === "student";
  const roleLabel =
    user?.role === "student" ? "Schüler"
    : user?.role === "teacher" ? "Lehrer"
    : user?.role === "school_admin" ? "Schul-Admin"
    : "Superadmin";

  useEffect(() => {
    if (!userMenuOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (userMenuRef.current && !userMenuRef.current.contains(event.target as Node)) {
        setUserMenuOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setUserMenuOpen(false);
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [userMenuOpen]);

  const handleLogout = async () => {
    setUserMenuOpen(false);
    await logout();
    navigate("/login");
  };

  const goTo = (to: string) => {
    setUserMenuOpen(false);
    navigate(to);
  };

  const renderItem = (item: NavItem) => (
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
  );

  const sections: NavSection[] = [
    { title: "", items: dashboardItems },
    { title: "Schule", items: schoolItems },
    { title: "Lernen", items: learnItems },
    ...(adminItems.length > 0 ? [{ title: "Verwaltung", items: adminItems } satisfies NavSection] : []),
  ].filter((section) => section.items.length > 0);

  return (
    <aside className="sidebar sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
      <div className="sidebar-header flex h-20 items-center gap-3 border-b border-gray-200 px-6 dark:border-gray-700">
        <span className="font-bold tracking-tight text-gray-900 dark:text-white">SmartTable</span>
        <span className="ml-auto flex items-center gap-1">
          <ThemeToggle />
        </span>
      </div>

      <nav className="flex-1 overflow-y-auto p-4">
        {sections.map((section) => (
          <div key={section.title || "start"} className="mb-5 last:mb-0">
            {section.title && (
              <p className="mb-2 px-3 text-[11px] font-bold uppercase tracking-wider text-gray-400 dark:text-gray-500">
                {section.title}
              </p>
            )}
            <ul className="flex flex-col gap-1">
              {section.items.map(renderItem)}
            </ul>
          </div>
        ))}
        {provider === "schoolconnect" && (
          <p className="mt-3 rounded-lg bg-indigo-50 px-3 py-2 text-[11px] font-medium text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300">
            Datenquelle: SchoolConnect
          </p>
        )}
      </nav>

      <div ref={userMenuRef} className="sidebar-user relative border-t border-gray-200 p-4 dark:border-gray-700">
        {userMenuOpen && (
          <div
            role="menu"
            aria-label="Benutzermenü"
            className="absolute inset-x-4 bottom-full mb-2 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
          >
            <div className="border-b border-gray-100 px-4 py-3 dark:border-gray-700">
              <p className="truncate text-sm font-medium text-gray-900 dark:text-white">
                {user?.first_name} {user?.last_name}
              </p>
              <p className="truncate text-xs text-gray-500 dark:text-gray-400">{roleLabel}</p>
            </div>
            <button
              type="button"
              role="menuitem"
              onClick={() => goTo("/settings")}
              className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
            >
              <GearIcon />
              Einstellungen
            </button>
            {canJoin && (
              <button
                type="button"
                role="menuitem"
                onClick={() => goTo("/join")}
                className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
              >
                <JoinIcon />
                Klasse beitreten
              </button>
            )}
            <button
              type="button"
              role="menuitem"
              onClick={handleLogout}
              className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/40"
            >
              <LogoutIcon />
              Abmelden
            </button>
          </div>
        )}
        <button
          type="button"
          onClick={() => setUserMenuOpen((open) => !open)}
          aria-haspopup="menu"
          aria-expanded={userMenuOpen}
          title="Benutzermenü öffnen"
          className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          <div className="flex size-8 items-center justify-center rounded-full bg-indigo-100 text-sm font-medium text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300">
            {user?.first_name?.[0]}{user?.last_name?.[0]}
          </div>
          <div className="flex-1 overflow-hidden">
            <p className="truncate text-sm font-medium text-gray-900 dark:text-white">
              {user?.first_name} {user?.last_name}
            </p>
            <p className="truncate text-xs text-gray-500 dark:text-gray-400">
              {roleLabel}
            </p>
          </div>
          <span className={`text-gray-400 transition-transform ${userMenuOpen ? "" : "rotate-180"}`}>
            <ChevronUpIcon />
          </span>
        </button>
      </div>
    </aside>
  );
}
