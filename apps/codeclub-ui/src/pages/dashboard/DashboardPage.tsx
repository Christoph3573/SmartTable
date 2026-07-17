import { useAuthStore } from "../../store/authStore";
import { Badge } from "../../components/ui/Badge";

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
          icon="🔄"
          href="/substitutions"
        />
        <DashboardCard
          title="Termine"
          description="Nächster Termin in 3 Tagen"
          icon="📅"
          href="/calendar"
        />
        <DashboardCard
          title="Hausaufgaben"
          description="2 offene Aufgaben"
          icon="📚"
          href="/homework"
        />
        <DashboardCard
          title="Dateien"
          description="Neue Materialien verfügbar"
          icon="📁"
          href="/files"
        />
        <DashboardCard
          title="Chat"
          description="3 ungelesene Nachrichten"
          icon="💬"
          href="/chat"
        />
      </div>
    </div>
  );
}

type CardProps = {
  title: string;
  description: string;
  icon: string;
  href: string;
};

function DashboardCard({ title, description, icon, href }: CardProps) {
  return (
    <a
      href={href}
      className="flex items-start gap-4 rounded-xl border border-gray-200 bg-white p-5 transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-900"
    >
      <span className="text-2xl">{icon}</span>
      <div>
        <p className="font-medium text-gray-900 dark:text-white">{title}</p>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{description}</p>
      </div>
    </a>
  );
}
