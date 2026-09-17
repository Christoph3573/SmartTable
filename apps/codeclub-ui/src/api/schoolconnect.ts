import { apiClient } from "./client";

/**
 * SchoolConnect-Integration (v0.1.0, "schoolconnect serve").
 *
 * Das SmartTable-Backend proxied lesende Plugin-Funktionen unter
 * `/api/v1/integrations/schoolconnect/...` — hier liegen nur die
 * Frontend-Typen + Mapping-Hilfen auf die SmartTable-Formate.
 */

export type SchoolConnectPluginId =
  | "schuelerportal"
  | "mebis"
  | "bycs-drive"
  | "lernplan-bayern";

export type SchoolConnectPluginInfo = {
  id: string;
  name: string;
  needs_auth: boolean;
  functions: string[];
};

export type SchoolConnectStatus = {
  configured: boolean;
  reachable: boolean;
  base_url?: string;
  hint?: string;
  plugins?: SchoolConnectPluginInfo[];
};

type SchoolConnectResult<T> = {
  plugin: string;
  function: string;
  data: T;
};

/** Stundenplan-Eintrag: {tag, day 0-4 (Mo-Fr), stunde, zeit, kurs, raum}. */
export type SchuelerportalTimetableEntry = {
  tag?: string;
  day: number;
  stunde: number;
  zeit?: string;
  kurs: string;
  raum?: string;
};

export type SchuelerportalTimetable = {
  schule?: string;
  count: number;
  eintraege: SchuelerportalTimetableEntry[];
  zeittafel?: { hour: number; value: string; name: string }[];
};

/** Vertretungs-Eintrag: {date, hour, class, uf, vertr_uf, room, reason, ...}. */
export type SchuelerportalSubstitution = Record<string, unknown>;

export type SchuelerportalVertretungsplan = {
  schule?: string;
  count: number;
  eintraege: SchuelerportalSubstitution[];
  mitteilungen?: Record<string, unknown>[];
};

export type SchuelerportalHomework = {
  schule?: string;
  count: number;
  aufgaben: Record<string, unknown>[];
};

export const NEEDS_AUTH_PLUGINS: SchoolConnectPluginId[] = [
  "schuelerportal",
  "mebis",
  "bycs-drive",
];

export const PLUGIN_NAMES: Record<SchoolConnectPluginId, string> = {
  schuelerportal: "Schülerportal",
  mebis: "mebis (ByCS-Lernplattform)",
  "bycs-drive": "ByCS Drive (Dateicloud)",
  "lernplan-bayern": "Lernplan Bayern (LehrplanPLUS)",
};

export const schoolConnectApi = {
  status: () =>
    apiClient
      .get<SchoolConnectStatus>("/api/v1/integrations/schoolconnect/status")
      .then((r) => r.data),

  auth: (plugin: string, credentials: Record<string, string>) =>
    apiClient
      .post<{ stored: string[] }>(
        `/api/v1/integrations/schoolconnect/${plugin}/auth`,
        credentials
      )
      .then((r) => r.data),

  logout: (plugin: string) =>
    apiClient
      .post(`/api/v1/integrations/schoolconnect/${plugin}/logout`)
      .then((r) => r.data),

  call: <T>(plugin: string, fn: string, params?: Record<string, string>) =>
    apiClient
      .get<SchoolConnectResult<T>>(
        `/api/v1/integrations/schoolconnect/${plugin}/${fn}`,
        { params }
      )
      .then((r) => r.data.data),

  stundenplan: (params?: { tag?: string; uf?: string }) =>
    schoolConnectApi.call<SchuelerportalTimetable>(
      "schuelerportal",
      "stundenplan",
      params
    ),

  vertretungsplan: (params?: { datum?: string }) =>
    schoolConnectApi.call<SchuelerportalVertretungsplan>(
      "schuelerportal",
      "vertretungsplan",
      params
    ),

  hausaufgaben: () =>
    schoolConnectApi.call<SchuelerportalHomework>(
      "schuelerportal",
      "hausaufgaben"
    ),
};

/** Wochentag 0-4 (Mo-Fr, SchoolConnect) -> 1-5 (Mo-Fr, SmartTable). */
export function scDayToSmartTable(day: number): number {
  return day + 1;
}

const DAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"];

/** Kurskürzel des Schülerportals (z.B. "2m") auf eine lesbare Fach-Zeile mappen. */
export function scTimetableEntryLabel(entry: SchuelerportalTimetableEntry): {
  dayOfWeek: number;
  period: number;
  subject: string;
  room: string;
} {
  return {
    dayOfWeek: scDayToSmartTable(entry.day),
    period: entry.stunde,
    subject:
      entry.kurs || entry.tag || DAY_LABELS[entry.day] || "Unterricht",
    room: entry.raum ? `Raum ${entry.raum}` : "Raum folgt",
  };
}

/** Vertretungs-Eintrag -> Anzeige-Felder (defensiv, Felder sind untypisiert). */
export function scSubstitutionLabel(entry: SchuelerportalSubstitution): {
  period: number | string;
  title: string;
  detail: string;
} {
  const str = (key: string): string => {
    const value = entry[key];
    return typeof value === "string" || typeof value === "number"
      ? String(value)
      : "";
  };
  const period = str("hour") || str("stunde") || str("period") || "—";
  const course = str("uf") || str("kurs") || str("fach") || str("class");
  const replacement = str("vertr_uf") || str("vertretung");
  const room = str("room") || str("raum");
  const reason = str("reason") || str("grund") || str("text") || str("note");
  const title = replacement
    ? `Vertretung ${course ? `(${course} → ${replacement})` : ""}`.trim()
    : course || "Unterricht geändert";
  const detail = [room ? `Raum ${room}` : "", reason].filter(Boolean).join(" · ");
  return { period, title, detail: detail || "Details im Vertretungsplan" };
}
