import { create } from "zustand";

export type DataProvider = "smarttable" | "schoolconnect";

const STORAGE_KEY = "smarttable-data-provider";

function getInitialProvider(): DataProvider {
  if (typeof window === "undefined") return "smarttable";
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "smarttable" || stored === "schoolconnect") return stored;
    return "smarttable";
  } catch {
    return "smarttable";
  }
}

type SettingsState = {
  /** Aktiver Daten-Provider für Stundenplan/Hausaufgaben/Vertretungen. */
  provider: DataProvider;
  setProvider: (provider: DataProvider) => void;
};

export const useSettingsStore = create<SettingsState>((set) => ({
  provider: getInitialProvider(),

  setProvider: (provider) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, provider);
    } catch {
      // localStorage nicht verfügbar -> nur Session anwenden
    }
    set({ provider });
  },
}));
