import { create } from "zustand";
import { authApi, type User } from "../api/auth";
import { setAccessToken } from "../api/client";

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
      set({ user: null });
    }
  },

  initialize: async () => {
    try {
      const res = await authApi.refresh();
      setAccessToken(res.data.access_token);
      const meRes = await authApi.me();
      set({ user: meRes.data, isInitialized: true });
    } catch {
      set({ user: null, isInitialized: true });
    }
  },
}));
