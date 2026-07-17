import { apiClient } from "./client";

export type Role = "student" | "teacher" | "admin";

export type User = {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  role: Role;
};

export type LoginRequest = {
  email: string;
  password: string;
};

export type LoginResponse = {
  access_token: string;
  user: User;
};

export const authApi = {
  login: (data: LoginRequest) =>
    apiClient.post<LoginResponse>("/api/v1/auth/login", data),

  logout: () => apiClient.post("/api/v1/auth/logout"),

  refresh: () =>
    apiClient.post<{ access_token: string }>("/api/v1/auth/refresh"),

  me: () => apiClient.get<User>("/api/v1/auth/me"),

  updateMe: (data: Partial<Pick<User, "first_name" | "last_name" | "email">>) =>
    apiClient.patch<User>("/api/v1/auth/me", data),
};
