import { create } from "zustand";

export type DataSource = "proprietary" | "schoolconnect";

const STORAGE_KEY = "smarttable-data-source";

function getInitialDataSource(): DataSource {
  if (typeof window === "undefined") return "proprietary";
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "proprietary" || stored === "schoolconnect") return stored;
  } catch {
    // localStorage nicht verfügbar -> Default verwenden
  }
  return "proprietary";
}

type SettingsState = {
  dataSource: DataSource;
  setDataSource: (source: DataSource) => void;
};

export const useSettingsStore = create<SettingsState>((set) => ({
  dataSource: getInitialDataSource(),

  setDataSource: (source) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, source);
    } catch {
      // nur Session anwenden, wenn localStorage nicht verfügbar ist
    }
    set({ dataSource: source });
    // Hinweis: Die Auswahl bleibt vorerst ohne Effekt (kein Backend-Aufruf,
    // keine Änderung der Datenabfragen).
  },
}));
