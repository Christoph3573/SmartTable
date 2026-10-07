import { Outlet } from "react-router-dom";
import { ThemeToggle } from "../components/ui/ThemeToggle";

export function AuthLayout() {
  return (
    <div className="relative min-h-screen bg-cover bg-center bg-[url('/login-bg.jpg')] md:bg-[position:70%_center]">
      <div aria-hidden className="absolute inset-0 bg-black/35 dark:bg-black/55" />
      <div className="absolute right-4 top-4 z-10">
        <ThemeToggle className="border border-white/30 bg-white/85 shadow-sm backdrop-blur" />
      </div>
      <div className="relative z-[1] flex min-h-screen items-center justify-center p-4 md:justify-start md:p-12 lg:p-16">
        <Outlet />
      </div>
    </div>
  );
}
