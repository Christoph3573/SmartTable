import { useAuthStore } from "../../store/authStore";
import { Badge } from "../../components/ui/Badge";
import type { ReactNode } from "react";

export function DashboardPage() {
  const user = useAuthStore((s) => s.user);

  const roleLabel = user?.role === "student" ? "Schüler" : user?.role === "teacher" ? "Lehrer" : "Administrator";
  const roleVariant = user?.role === "admin" ? "error" : user?.role === "teacher" ? "info" : "success";

  return (
    <div className="p-8">
      <div className="mb-8">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
            Guten Tag, {user?.first_name}!
          </h1>
          <Badge variant={roleVariant}>{roleLabel}</Badge>
        </div>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Hier ist deine Übersicht für heute.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <DashboardCard
          title="Vertretungsplan"
          description="Keine Vertretungen für heute"
          href="/substitutions"
        >
          <SwapIcon />
        </DashboardCard>
        <DashboardCard
          title="Termine"
          description="Nächster Termin in 3 Tagen"
          href="/calendar"
        >
          <CalendarIcon />
        </DashboardCard>
        <DashboardCard
          title="Hausaufgaben"
          description="2 offene Aufgaben"
          href="/homework"
        >
          <BookIcon />
        </DashboardCard>
        <DashboardCard
          title="Dateien"
          description="Neue Materialien verfügbar"
          href="/files"
        >
          <FolderIcon />
        </DashboardCard>
        <DashboardCard
          title="Chat"
          description="3 ungelesene Nachrichten"
          href="/chat"
        >
          <ChatIcon />
        </DashboardCard>
      </div>
    </div>
  );
}

type CardProps = {
  title: string;
  description: string;
  href: string;
  children: ReactNode;
};

function DashboardCard({ title, description, href, children }: CardProps) {
  return (
    <a
      href={href}
      className="flex items-start gap-4 rounded-xl border border-gray-200 bg-white p-5 transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-900"
    >
      <span className="size-8 text-gray-600 dark:text-gray-400">{children}</span>
      <div>
        <p className="font-medium text-gray-900 dark:text-white">{title}</p>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{description}</p>
      </div>
    </a>
  );
}

function SwapIcon() {
  return (
    <svg className="size-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
    </svg>
  );
}

function CalendarIcon() {
  return (
    <svg className="size-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
    </svg>
  );
}

function BookIcon() {
  return (
    <svg className="size-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 0 0 6 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 0 1 6 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 0 1 6-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0 0 18 18a8.967 8.967 0 0 0-6 2.292m0-14.25v14.25" />
    </svg>
  );
}

function FolderIcon() {
  return (
    <svg className="size-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
    </svg>
  );
}

function ChatIcon() {
  return (
    <svg className="size-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
    </svg>
  );
}
