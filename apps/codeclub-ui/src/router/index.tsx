import { createBrowserRouter, Navigate } from "react-router-dom";
import { AuthLayout } from "../layouts/AuthLayout";
import { AppLayout } from "../layouts/AppLayout";
import { ProtectedRoute } from "./ProtectedRoute";
import { RoleRoute } from "./RoleRoute";
import { LoginPage } from "../pages/auth/LoginPage";
import { RegisterPage } from "../pages/auth/RegisterPage";
import { DashboardPage } from "../pages/dashboard/DashboardPage";
import { SubstitutionsPage } from "../pages/substitutions/SubstitutionsPage";
import { CalendarPage } from "../pages/calendar/CalendarPage";
import { FilesPage } from "../pages/files/FilesPage";
import { HomeworkPage } from "../pages/homework/HomeworkPage";
import { TimetablePage } from "../pages/timetable/TimetablePage";
import { ChatPage } from "../pages/chat/ChatPage";
import { SettingsPage } from "../pages/settings/SettingsPage";
import { UsersPage } from "../pages/admin/UsersPage";
import { ClassesPage } from "../pages/admin/ClassesPage";
import { SchoolsPage } from "../pages/admin/SchoolsPage";
import { JoinClassPage } from "../pages/student/JoinClassPage";

export const router = createBrowserRouter([
  {
    element: <AuthLayout />,
    children: [
      { path: "/login", element: <LoginPage /> },
      { path: "/register", element: <RegisterPage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: "/dashboard", element: <DashboardPage /> },
          { path: "/timetable", element: <TimetablePage /> },
          { path: "/substitutions", element: <SubstitutionsPage /> },
          { path: "/calendar", element: <CalendarPage /> },
          { path: "/files", element: <FilesPage /> },
          { path: "/homework", element: <HomeworkPage /> },
          { path: "/chat", element: <ChatPage /> },
          { path: "/settings", element: <SettingsPage /> },
          {
            element: <RoleRoute allowed={["student"]} />,
            children: [
              { path: "/join", element: <JoinClassPage /> },
            ],
          },
          {
            element: <RoleRoute allowed={["superadmin", "admin", "school_admin"]} />,
            children: [
              { path: "/admin/users", element: <UsersPage /> },
            ],
          },
          {
            element: <RoleRoute allowed={["superadmin", "admin", "school_admin", "teacher"]} />,
            children: [
              { path: "/admin/classes", element: <ClassesPage /> },
            ],
          },
          {
            element: <RoleRoute allowed={["superadmin", "admin"]} />,
            children: [
              { path: "/admin/schools", element: <SchoolsPage /> },
            ],
          },
        ],
      },
    ],
  },
  { path: "/", element: <Navigate to="/dashboard" replace /> },
  { path: "*", element: <Navigate to="/dashboard" replace /> },
]);
