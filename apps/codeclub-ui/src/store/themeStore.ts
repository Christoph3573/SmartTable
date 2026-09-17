import { create } from "zustand";

export type Theme = "light" | "dark";
export type ThemeMode = "light" | "dark" | "system";

const STORAGE_KEY = "smarttable-theme";

function getStoredMode(): ThemeMode | null {
  if (typeof window === "undefined") return null;
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") return stored;
    return null;
  } catch {
    return null;
  }
}

function systemTheme(): Theme {
  if (typeof window === "undefined") return "light";
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  } catch {
    return "light";
  }
}

function resolveTheme(mode: ThemeMode): Theme {
  return mode === "system" ? systemTheme() : mode;
}

function applyTheme(theme: Theme) {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

type ThemeState = {
  /** Aufgelöstes, tatsächlich angewendetes Theme (für Styles/Toggle). */
  theme: Theme;
  /** Gewählter Modus — "system" folgt dem Betriebssystem live. */
  mode: ThemeMode;
  init: () => void;
  setMode: (mode: ThemeMode) => void;
  /** Explizite Wahl (light/dark) — entspricht setMode mit demselben Wert. */
  setTheme: (theme: Theme) => void;
  toggle: () => void;
};

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: resolveTheme(getStoredMode() ?? "system"),
  mode: getStoredMode() ?? "system",

  init: () => {
    const mode = getStoredMode() ?? "system";
    const theme = resolveTheme(mode);
    applyTheme(theme);
    set({ theme, mode });

    // Systemwechsel live übernehmen, solange "system" gewählt ist.
    try {
      const media = window.matchMedia("(prefers-color-scheme: dark)");
      const onChange = (event: MediaQueryListEvent) => {
        if (get().mode !== "system") return;
        const next: Theme = event.matches ? "dark" : "light";
        applyTheme(next);
        set({ theme: next });
      };
      media.addEventListener("change", onChange);
    } catch {
      // ältere Browser ignorieren
    }
  },

  setMode: (mode) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, mode);
    } catch {
      // localStorage nicht verfügbar (privater Modus) -> nur Session anwenden
    }
    const theme = resolveTheme(mode);
    applyTheme(theme);
    set({ theme, mode });
  },

  setTheme: (theme) => {
    get().setMode(theme);
  },

  toggle: () => {
    get().setMode(get().theme === "dark" ? "light" : "dark");
  },
}));
