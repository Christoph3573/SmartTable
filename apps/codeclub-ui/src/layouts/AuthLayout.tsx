import { Outlet } from "react-router-dom";

export function AuthLayout() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f7f7f2] p-4">
      <Outlet />
    </div>
  );
}
