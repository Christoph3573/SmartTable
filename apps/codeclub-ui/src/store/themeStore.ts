import { create } from "zustand";

type Theme = "light" | "dark";

const STORAGE_KEY = "smarttable-theme";

function getInitialTheme(): Theme {
  if (typeof window === "undefined") return "light";
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  } catch {
    return "light";
  }
}

function applyTheme(theme: Theme) {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

type ThemeState = {
  theme: Theme;
  init: () => void;
  setTheme: (theme: Theme) => void;
  toggle: () => void;
};

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: getInitialTheme(),

  init: () => {
    const theme = getInitialTheme();
    applyTheme(theme);
    set({ theme });

    // Systemwechsel live übernehmen, solange der User nichts manuell gewählt hat
    try {
      const media = window.matchMedia("(prefers-color-scheme: dark)");
      const onChange = (event: MediaQueryListEvent) => {
        if (window.localStorage.getItem(STORAGE_KEY)) return;
        const next: Theme = event.matches ? "dark" : "light";
        applyTheme(next);
        set({ theme: next });
      };
      media.addEventListener("change", onChange);
    } catch {
      // ältere Browser ignorieren
    }
  },

  setTheme: (theme) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // localStorage nicht verfügbar (privater Modus) -> nur Session anwenden
    }
    applyTheme(theme);
    set({ theme });
  },

  toggle: () => {
    get().setTheme(get().theme === "dark" ? "light" : "dark");
  },
}));
