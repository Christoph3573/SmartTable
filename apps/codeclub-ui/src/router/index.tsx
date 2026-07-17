import { createBrowserRouter, Navigate } from "react-router-dom";
import { AuthLayout } from "../layouts/AuthLayout";
import { AppLayout } from "../layouts/AppLayout";
import { ProtectedRoute } from "./ProtectedRoute";
import { RoleRoute } from "./RoleRoute";
import { LoginPage } from "../pages/auth/LoginPage";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { SubstitutionsPage } from "../pages/substitutions/SubstitutionsPage";
import { CalendarPage } from "../pages/calendar/CalendarPage";
import { FilesPage } from "../pages/files/FilesPage";
import { HomeworkPage } from "../pages/homework/HomeworkPage";
import { ChatPage } from "../pages/chat/ChatPage";
import { UsersPage } from "../pages/admin/UsersPage";
import { ClassesPage } from "../pages/admin/ClassesPage";

export const router = createBrowserRouter([
  {
    element: <AuthLayout />,
    children: [
      { path: "/login", element: <LoginPage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: "/dashboard", element: <DashboardPage /> },
          { path: "/substitutions", element: <SubstitutionsPage /> },
          { path: "/calendar", element: <CalendarPage /> },
          { path: "/files", element: <FilesPage /> },
          { path: "/homework", element: <HomeworkPage /> },
          { path: "/chat", element: <ChatPage /> },
          {
            element: <RoleRoute allowed={["admin"]} />,
            children: [
              { path: "/admin/users", element: <UsersPage /> },
            ],
          },
          {
            element: <RoleRoute allowed={["admin", "teacher"]} />,
            children: [
              { path: "/admin/classes", element: <ClassesPage /> },
            ],
          },
        ],
      },
    ],
  },
  { path: "/", element: <Navigate to="/dashboard" replace /> },
  { path: "*", element: <Navigate to="/dashboard" replace /> },
]);
