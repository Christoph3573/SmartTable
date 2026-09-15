import { Outlet } from "react-router-dom";
import { ThemeToggle } from "../components/ui/ThemeToggle";

export function AuthLayout() {
  return (
    <div className="relative flex min-h-screen items-center justify-center bg-[#f7f7f2] p-4 dark:bg-gray-950">
      <div className="absolute right-4 top-4">
        <ThemeToggle />
      </div>
      <Outlet />
    </div>
  );
}
