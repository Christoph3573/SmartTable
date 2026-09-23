/* eslint-disable react-refresh/only-export-components */
import { StrictMode, useEffect } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import "./index.css";
import { router } from "./router";
import { queryClient } from "./api/queryClient";
import { useAuthStore } from "./store/authStore";
import { useThemeStore } from "./store/themeStore";

function AppRoot() {
  const initialize = useAuthStore((s) => s.initialize);
  const initTheme = useThemeStore((s) => s.init);

  useEffect(() => {
    initTheme();
    initialize();
  }, [initTheme, initialize]);

  return <RouterProvider router={router} />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AppRoot />
    </QueryClientProvider>
  </StrictMode>
);
