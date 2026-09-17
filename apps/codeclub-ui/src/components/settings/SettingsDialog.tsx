import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useThemeStore, type ThemeMode } from "../../store/themeStore";
import { useSettingsStore, type DataProvider } from "../../store/settingsStore";
import {
  NEEDS_AUTH_PLUGINS,
  PLUGIN_NAMES,
  schoolConnectApi,
  type SchoolConnectPluginId,
} from "../../api/schoolconnect";
import { Button } from "../ui/Button";
import { EmptyState, ErrorState, LoadingState } from "../ui/Page";

const PROVIDER_OPTIONS: { value: DataProvider; title: string; description: string }[] = [
  {
    value: "smarttable",
    title: "SmartTable",
    description: "Eigene Schul-App: Stundenplan, Vertretungen und Hausaufgaben aus deiner Schule.",
  },
  {
    value: "schoolconnect",
    title: "SchoolConnect",
    description: "Externe Schulplattformen (Schülerportal, mebis, ByCS Drive, LehrplanPLUS) via SchoolConnect-API v0.1.0.",
  },
];

const THEME_OPTIONS: { value: ThemeMode; title: string; description: string }[] = [
  { value: "light", title: "Hell", description: "Immer helles Farbschema." },
  { value: "dark", title: "Dunkel", description: "Immer dunkles Farbschema." },
  { value: "system", title: "System", description: "Folgt automatisch dem Betriebssystem." },
];

export function SettingsDialog({ onClose }: { onClose: () => void }) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="modal-backdrop"
      role="dialog"
      aria-modal="true"
      aria-label="Einstellungen"
      onClick={onClose}
    >
      <div className="modal max-w-xl" onClick={(event) => event.stopPropagation()}>
        <h2>Einstellungen</h2>
        <SettingsContent />
        <div className="modal-actions">
          <Button onClick={onClose}>Fertig</Button>
        </div>
      </div>
    </div>
  );
}

export function SettingsContent() {
  const { mode, setMode } = useThemeStore();
  const { provider, setProvider } = useSettingsStore();

  return (
    <>
      <section aria-label="Darstellung">
          <h3 className="mb-2 text-sm font-bold text-gray-900 dark:text-white">Darstellung</h3>
          <div className="grid gap-2 sm:grid-cols-3" role="radiogroup" aria-label="Farbschema">
            {THEME_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={mode === option.value}
                onClick={() => setMode(option.value)}
                className={`rounded-xl border p-3 text-left transition-colors ${
                  mode === option.value
                    ? "border-indigo-500 bg-indigo-50 dark:border-indigo-400 dark:bg-indigo-900/30"
                    : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700"
                }`}
              >
                <span className="block text-sm font-bold text-gray-900 dark:text-white">
                  {option.title}
                </span>
                <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                  {option.description}
                </span>
              </button>
            ))}
          </div>
        </section>

        <section className="mt-5" aria-label="Datenquelle">
          <h3 className="mb-2 text-sm font-bold text-gray-900 dark:text-white">Datenquelle</h3>
          <div className="grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label="Data Provider">
            {PROVIDER_OPTIONS.map((option) => (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={provider === option.value}
                onClick={() => setProvider(option.value)}
                className={`rounded-xl border p-3 text-left transition-colors ${
                  provider === option.value
                    ? "border-indigo-500 bg-indigo-50 dark:border-indigo-400 dark:bg-indigo-900/30"
                    : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700"
                }`}
              >
                <span className="block text-sm font-bold text-gray-900 dark:text-white">
                  {option.title}
                </span>
                <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                  {option.description}
                </span>
              </button>
            ))}
          </div>
          <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            Gilt für Stundenplan, Vertretungsplan und Hausaufgaben. Chat, Kalender und Dateien bleiben immer bei SmartTable.
          </p>
        </section>

        {provider === "schoolconnect" && (
          <section className="mt-5" aria-label="SchoolConnect-Verbindung">
            <h3 className="mb-2 text-sm font-bold text-gray-900 dark:text-white">
              SchoolConnect-Verbindung
            </h3>
            <SchoolConnectPanel />
          </section>
        )}
    </>
  );
}

function SchoolConnectPanel() {
  const client = useQueryClient();
  const status = useQuery({
    queryKey: ["schoolconnect", "status"],
    queryFn: schoolConnectApi.status,
  });
  const logout = useMutation({
    mutationFn: (plugin: string) => schoolConnectApi.logout(plugin),
    onSuccess: () => client.invalidateQueries({ queryKey: ["schoolconnect"] }),
  });

  if (status.isLoading) return <LoadingState label="SchoolConnect wird geprüft" />;
  if (status.isError || !status.data) {
    return <ErrorState onRetry={() => status.refetch()} message="SchoolConnect konnte nicht erreicht werden." />;
  }
  if (!status.data.reachable) {
    return (
      <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
        <strong className="block">SchoolConnect läuft nicht.</strong>
        <p className="mt-1">{status.data.hint ?? "Bitte die SchoolConnect-REST-API starten."}</p>
        <p className="mt-2 font-mono text-xs">
          Binary aus v0.1.0 laden, dann: REST_ADDR=:8081 schoolconnect serve
        </p>
        <a
          className="mt-2 inline-block font-semibold underline"
          href="https://github.com/Christoph3573/SchoolConnect/releases/tag/v0.1.0"
          target="_blank"
          rel="noreferrer"
        >
          Release v0.1.0 öffnen
        </a>
        <div className="mt-3">
          <Button size="sm" variant="secondary" onClick={() => status.refetch()}>
            Erneut prüfen
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {(status.data.plugins ?? []).length === 0 && (
        <EmptyState
          title="Keine Plugins gefunden"
          description="SchoolConnect antwortet, meldet aber keine Plugins."
        />
      )}
      {(status.data.plugins ?? []).map((plugin) => (
        <PluginCard
          key={plugin.id}
          pluginId={plugin.id as SchoolConnectPluginId}
          needsAuth={(plugin.needs_auth || NEEDS_AUTH_PLUGINS.includes(plugin.id as SchoolConnectPluginId))}
          loggingOut={logout.isPending}
          onLogout={() => logout.mutate(plugin.id)}
        />
      ))}
      {logout.isError && (
        <p className="text-sm text-red-600">Abmelden fehlgeschlagen — bitte erneut versuchen.</p>
      )}
    </div>
  );
}

function PluginCard({
  pluginId,
  needsAuth,
  loggingOut,
  onLogout,
}: {
  pluginId: SchoolConnectPluginId;
  needsAuth: boolean;
  loggingOut: boolean;
  onLogout: () => void;
}) {
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [schule, setSchule] = useState("indomagy");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<string | null>(null);

  const name = PLUGIN_NAMES[pluginId] ?? pluginId;

  const auth = useMutation({
    mutationFn: (credentials: Record<string, string>) =>
      schoolConnectApi.auth(pluginId, credentials),
    onSuccess: () => {
      setPassword("");
      setOpen(false);
      setCheckResult("Angemeldet — Sessions werden von SchoolConnect verwaltet.");
      client.invalidateQueries({ queryKey: ["schoolconnect"] });
    },
  });

  const checkSession = async () => {
    setChecking(true);
    setCheckResult(null);
    try {
      if (pluginId === "schuelerportal") {
        await schoolConnectApi.call("schuelerportal", "profil");
        setCheckResult("Verbunden — Profil konnte geladen werden.");
      } else if (pluginId === "lernplan-bayern") {
        await schoolConnectApi.call("lernplan-bayern", "search", {
          schulart: "Gymnasium",
          lehrplankapitel: "kap4",
          fach: "Deutsch",
          jahrgangsstufe: "9",
        });
        setCheckResult("Verbunden — LehrplanPLUS antwortet (kein Login nötig).");
      } else {
        await schoolConnectApi.status();
        setCheckResult("SchoolConnect antwortet. Bitte einmalig anmelden, dann prüfen.");
      }
    } catch {
      setCheckResult("Keine gültige Session — bitte anmelden.");
    } finally {
      setChecking(false);
    }
  };

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    const credentials: Record<string, string> =
      pluginId === "schuelerportal"
        ? { schule, username, password }
        : { username, password };
    auth.mutate(credentials);
  };

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <strong className="block truncate text-sm text-gray-900 dark:text-white">{name}</strong>
          <p className="font-mono text-xs text-gray-500 dark:text-gray-400">{pluginId}</p>
        </div>
        {!needsAuth ? (
          <span className="pill blue">Kein Login nötig</span>
        ) : (
          <div className="flex gap-2">
            <Button size="sm" variant="secondary" onClick={() => setOpen((v) => !v)}>
              {open ? "Schließen" : "Anmelden"}
            </Button>
            <Button size="sm" variant="ghost" loading={loggingOut} onClick={onLogout}>
              Abmelden
            </Button>
          </div>
        )}
      </div>

      <div className="mt-3 flex items-center gap-2">
        <Button size="sm" variant="ghost" loading={checking} onClick={checkSession}>
          Verbindung prüfen
        </Button>
        {checkResult && (
          <p className="text-xs text-gray-500 dark:text-gray-400">{checkResult}</p>
        )}
      </div>

      {open && needsAuth && (
        <form onSubmit={handleSubmit} className="mt-3 flex flex-col gap-2">
          {pluginId === "schuelerportal" && (
            <label className="text-xs text-gray-600 dark:text-gray-300">
              Schulkürzel
              <input
                className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm"
                value={schule}
                onChange={(event) => setSchule(event.target.value)}
                placeholder="z.B. indomagy"
                required
              />
            </label>
          )}
          <label className="text-xs text-gray-600 dark:text-gray-300">
            Benutzername {pluginId === "schuelerportal" ? "(E-Mail)" : "(Kennung)"}
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
            />
          </label>
          <label className="text-xs text-gray-600 dark:text-gray-300">
            Passwort
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
            />
          </label>
          {auth.isError && (
            <p className="text-xs text-red-600">
              Anmeldung fehlgeschlagen — Zugangsdaten prüfen.
            </p>
          )}
          <div className="flex justify-end">
            <Button size="sm" loading={auth.isPending}>
              Anmelden
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
