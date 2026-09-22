import { AxiosError } from "axios";
import { apiClient } from "./client";

export type ProbeMethod = "GET" | "POST";

/** Ergebnis eines einzelnen Diagnose-Requests (Rohantwort, kein Werfen). */
export type ProbeResult = {
  ok: boolean;
  status: number;
  statusText: string;
  durationMs: number;
  url: string;
  data: unknown;
};

export type SidecarKey = "opencode" | "schoolconnect";

export type SidecarDiagnosis = {
  key: SidecarKey;
  label: string;
  configured: boolean;
  reachable: boolean;
  baseUrl?: string;
  hint?: string;
  raw: unknown;
};

/**
 * Schickt eine beliebige GET/POST-Anfrage über denselben axios-Client wie die
 * App (Bearer-Token + Refresh) ans SmartTable-Backend und liefert Status,
 * Dauer und Rohantwort zurück — auch bei 4xx/5xx. Ein reiner Netzwerkfehler
 * ergibt Status 0 mit der Fehlermeldung als `data`.
 *
 * Hinweis: Die Sidecars (opencode/schoolconnect) sind bewusst nur im internen
 * Compose-Netz erreichbar. Der Browser kann sie nie direkt ansprechen — alle
 * Checks laufen daher über die Backend-Proxy-Endpunkte.
 */
export async function probe(
  path: string,
  method: ProbeMethod = "GET",
  body?: string
): Promise<ProbeResult> {
  const url = path.startsWith("/") ? path : `/${path}`;
  const started = performance.now();
  let payload: unknown;
  if (method === "POST" && body && body.trim()) {
    try {
      payload = JSON.parse(body);
    } catch {
      payload = body;
    }
  }
  const config = { validateStatus: () => true };
  try {
    const res =
      method === "POST"
        ? await apiClient.post(url, payload, config)
        : await apiClient.get(url, config);
    return {
      ok: res.status >= 200 && res.status < 300,
      status: res.status,
      statusText: res.statusText,
      durationMs: Math.round(performance.now() - started),
      url,
      data: res.data,
    };
  } catch (error) {
    const durationMs = Math.round(performance.now() - started);
    if (error instanceof AxiosError && error.response) {
      return {
        ok: false,
        status: error.response.status,
        statusText: error.response.statusText,
        durationMs,
        url,
        data: error.response.data,
      };
    }
    return {
      ok: false,
      status: 0,
      statusText: "Netzwerkfehler",
      durationMs,
      url,
      data: error instanceof Error ? error.message : String(error),
    };
  }
}

function toDiagnosis(
  key: SidecarKey,
  label: string,
  result: ProbeResult
): SidecarDiagnosis {
  const payload =
    result.data && typeof result.data === "object"
      ? (result.data as Record<string, unknown>)
      : {};
  return {
    key,
    label,
    configured: Boolean(payload.configured),
    reachable: Boolean(payload.reachable),
    baseUrl: typeof payload.base_url === "string" ? payload.base_url : undefined,
    hint: typeof payload.hint === "string" ? payload.hint : undefined,
    raw: result.data,
  };
}

/** Fragt beide Sidecar-Status-Endpunkte parallel ab. */
export async function diagnoseSidecars(): Promise<SidecarDiagnosis[]> {
  const [open, school] = await Promise.all([
    probe("/api/v1/integrations/opencode/status"),
    probe("/api/v1/integrations/schoolconnect/status"),
  ]);
  return [
    toDiagnosis("opencode", "OpenCode (KI-Lernchat)", open),
    toDiagnosis("schoolconnect", "SchoolConnect", school),
  ];
}
