import { create } from "zustand";
import { authApi, type User } from "../api/auth";
import { setAccessToken } from "../api/client";
import { queryClient } from "../api/queryClient";
import { providerApi } from "../api/provider";
import { useSettingsStore } from "./settingsStore";

type AuthState = {
  user: User | null;
  isLoading: boolean;
  isInitialized: boolean;

  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  initialize: () => Promise<void>;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: false,
  isInitialized: false,

  login: async (email, password) => {
    set({ isLoading: true });
    try {
      const res = await authApi.login({ email, password });
      setAccessToken(res.data.access_token);
      // Cache des vorherigen Accounts verwerfen, bevor die App ihn liest.
      queryClient.clear();
      set({ user: res.data.user, isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      setAccessToken(null);
      // Ohne Full-Reload würde die App sonst die gecachten Daten des
      // abgemeldeten Accounts weiter anzeigen (fremde Chats/Nachrichten).
      queryClient.clear();
      set({ user: null });
    }
  },

  initialize: async () => {
    try {
      const res = await authApi.refresh();
      setAccessToken(res.data.access_token);
      const meRes = await authApi.me();
      set({ user: meRes.data, isInitialized: true });

      // Server-Einstellung adoptieren (best effort), damit ein Login auf
      // einem neuen Gerät die dort gewählte Datenquelle übernimmt.
      try {
        const serverProvider = await providerApi.get();
        if (useSettingsStore.getState().provider !== serverProvider) {
          useSettingsStore.getState().setProvider(serverProvider);
        }
      } catch {
        // Server nicht erreichbar → lokale Wahl behalten
      }
    } catch {
      set({ user: null, isInitialized: true });
    }
  },
}));
