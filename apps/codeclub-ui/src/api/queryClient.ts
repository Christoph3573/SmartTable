import { QueryClient } from "@tanstack/react-query";

/**
 * Ein globaler React-Query-Client für die App. Er wird bewusst als Modul
 * exportiert (statt in main.tsx gekapselt), damit der Auth-Store ihn beim
 * Login/Logout leeren kann: Sonst bleiben die gecachten Daten des vorherigen
 * Accounts (z. B. Chats) nach einem Kontowechsel ohne Full-Reload sichtbar.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60,
      retry: 1,
    },
  },
});
