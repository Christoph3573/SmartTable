import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  diagnoseMcp,
  diagnoseSidecars,
  probe,
  type ProbeMethod,
  type ProbeResult,
  type SidecarDiagnosis,
} from "../../api/diagnostics";
import { Button } from "../ui/Button";

const QUICK_TESTS: { label: string; method: ProbeMethod; path: string; body?: string }[] = [
  { label: "OpenCode-Status", method: "GET", path: "/api/v1/integrations/opencode/status" },
  { label: "MCP-Server (KI-Tools)", method: "GET", path: "/api/v1/integrations/opencode/mcp-status" },
  { label: "OpenCode-Sessions", method: "GET", path: "/api/v1/integrations/opencode/sessions" },
  { label: "SchoolConnect-Status", method: "GET", path: "/api/v1/integrations/schoolconnect/status" },
  {
    label: "Lernplan (öffentlich)",
    method: "GET",
    path: "/api/v1/integrations/schoolconnect/lernplan-bayern/search",
  },
];

function pretty(data: unknown): string {
  if (typeof data === "string") return data;
  try {
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
}

function statusClasses(result: ProbeResult): string {
  if (result.status === 0) return "pill red";
  if (result.ok) return "pill blue";
  return "pill amber";
}

/**
 * Kleine Diagnose-Konsole für die Einstellungen: zeigt die Erreichbarkeit der
 * internen Sidecars (OpenCode/SchoolConnect, geprüft über die Backend-Proxy-
 * Endpunkte) und erlaubt eigene GET/POST-Anfragen an die Backend-API.
 */
export function DebugConsole() {
  const diagnosis = useQuery({
    queryKey: ["diagnostics", "sidecars"],
    queryFn: diagnoseSidecars,
  });

  const [method, setMethod] = useState<ProbeMethod>("GET");
  const [path, setPath] = useState("/api/v1/integrations/opencode/status");
  const [body, setBody] = useState("");
  const [result, setResult] = useState<ProbeResult | null>(null);
  const [sending, setSending] = useState(false);

  const send = async () => {
    if (!path.trim()) return;
    setSending(true);
    try {
      setResult(await probe(path.trim(), method, body));
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-2 sm:grid-cols-2">
        {diagnosis.isLoading &&
          [0, 1].map((i) => (
            <div key={i} className="rounded-xl border border-gray-200 p-3 text-sm dark:border-gray-700">
              <span className="text-gray-500 dark:text-gray-400">Prüfe …</span>
            </div>
          ))}
        {diagnosis.isError && (
          <p className="text-sm text-red-600">
            Diagnose fehlgeschlagen — das Backend ist nicht erreichbar.
          </p>
        )}
        {(diagnosis.data ?? []).map((item) => (
          <SidecarCard key={item.key} item={item} />
        ))}
      </div>

      <McpCard />

      <div className="flex items-center justify-between">
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Sidecars sind nur intern erreichbar — Checks laufen über das Backend.
        </p>
        <Button size="sm" variant="secondary" loading={diagnosis.isFetching} onClick={() => diagnosis.refetch()}>
          Erneut prüfen
        </Button>
      </div>

      <div className="rounded-xl border border-gray-200 p-3 dark:border-gray-700">
        <div className="flex flex-wrap gap-2">
          {QUICK_TESTS.map((test) => (
            <button
              key={test.path}
              type="button"
              onClick={() => {
                setMethod(test.method);
                setPath(test.path);
                setBody(test.body ?? "");
              }}
              className="rounded-full border border-gray-300 px-3 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-800"
            >
              {test.label}
            </button>
          ))}
        </div>

        <div className="mt-3 flex flex-wrap items-end gap-2">
          <label className="text-xs text-gray-600 dark:text-gray-300">
            Methode
            <select
              className="mt-1 block rounded-lg border border-gray-300 bg-white p-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              value={method}
              onChange={(event) => setMethod(event.target.value as ProbeMethod)}
            >
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
          </label>
          <label className="min-w-[16rem] flex-1 text-xs text-gray-600 dark:text-gray-300">
            Pfad
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2 font-mono text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              value={path}
              onChange={(event) => setPath(event.target.value)}
              spellCheck={false}
              onKeyDown={(event) => {
                if (event.key === "Enter" && method === "GET") void send();
              }}
            />
          </label>
          <Button size="sm" loading={sending} onClick={send}>
            Senden
          </Button>
        </div>

        {method === "POST" && (
          <label className="mt-2 block text-xs text-gray-600 dark:text-gray-300">
            JSON-Body
            <textarea
              className="mt-1 h-24 w-full rounded-lg border border-gray-300 p-2 font-mono text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              value={body}
              onChange={(event) => setBody(event.target.value)}
              spellCheck={false}
              placeholder='{"content":"Hallo"}'
            />
          </label>
        )}

        {result && (
          <div className="mt-3">
            <div className="flex items-center gap-2 text-xs">
              <span className={statusClasses(result)}>
                {result.status === 0 ? "Netzwerkfehler" : `${result.status} ${result.statusText}`}
              </span>
              <span className="text-gray-500 dark:text-gray-400">{result.durationMs} ms</span>
              <span className="font-mono text-gray-400 dark:text-gray-500">{result.url}</span>
            </div>
            <pre className="mt-2 max-h-72 overflow-auto rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-900">
              {pretty(result.data)}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}

function McpCard() {
  const mcp = useQuery({
    queryKey: ["diagnostics", "mcp"],
    queryFn: diagnoseMcp,
    retry: 0,
  });

  const statusLabel = mcp.isLoading
    ? "Prüfe …"
    : mcp.data?.connected
    ? "MCP verbunden"
    : mcp.data?.reachable
    ? "Nicht verbunden"
    : "Nicht erreichbar";
  const pill = mcp.data?.connected ? "pill blue" : mcp.isLoading ? "pill" : "pill red";

  return (
    <div className="rounded-xl border border-gray-200 p-3 dark:border-gray-700">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <strong className="block truncate text-sm text-gray-900 dark:text-white">
            KI-Tools (MCP-Server smarttable)
          </strong>
          <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            Prüft, ob OpenCode den Backend-MCP-Server erreicht — darüber bekommt die Lern-KI
            Stundenplan, Vertretungen, Hausaufgaben und Vokabeln.
          </p>
        </div>
        <span className={pill}>{statusLabel}</span>
      </div>
      {mcp.data?.mcp_url && (
        <p className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{mcp.data.mcp_url}</p>
      )}
      {!mcp.isLoading && !mcp.data?.connected && mcp.data && (
        <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">
          {mcp.data.hint ??
            "Der MCP-Server ist aus dem opencode-Container nicht erreichbar. Prüfen: gleiche Compose-Netz, OPENCODE_MCP_URL=http://backend:8080/api/v1/mcp."}
        </p>
      )}
      <div className="mt-2 flex items-center gap-2">
        <Button size="sm" variant="secondary" loading={mcp.isFetching} onClick={() => mcp.refetch()}>
          MCP-Verbindung testen
        </Button>
      </div>
      {mcp.data?.servers && (
        <pre className="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-2 text-xs dark:bg-gray-900">
          {pretty(mcp.data.servers)}
        </pre>
      )}
    </div>
  );
}

function SidecarCard({ item }: { item: SidecarDiagnosis }) {
  const reachable = item.reachable;
  const pill = reachable ? "pill blue" : "pill red";
  const label = reachable ? "Erreichbar" : "Nicht erreichbar";
  return (
    <div className="rounded-xl border border-gray-200 p-3 dark:border-gray-700">
      <div className="flex items-center justify-between gap-2">
        <strong className="truncate text-sm text-gray-900 dark:text-white">{item.label}</strong>
        <span className={pill}>{label}</span>
      </div>
      {item.baseUrl && (
        <p className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{item.baseUrl}</p>
      )}
      {!reachable && item.hint && (
        <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">{item.hint}</p>
      )}
      <details className="mt-2">
        <summary className="cursor-pointer text-xs text-gray-500 dark:text-gray-400">
          Roantwort
        </summary>
        <pre className="mt-2 max-h-56 overflow-auto rounded-lg bg-gray-50 p-2 text-xs dark:bg-gray-900">
          {pretty(item.raw)}
        </pre>
      </details>
    </div>
  );
}
