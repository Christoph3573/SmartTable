import { apiClient } from "./client";

export type DataProvider = "smarttable" | "schoolconnect";

export const providerApi = {
  get: () =>
    apiClient
      .get<{ provider: DataProvider }>("/api/v1/me/provider")
      .then((r) => r.data.provider),
  set: (provider: DataProvider) =>
    apiClient
      .patch<{ provider: DataProvider }>("/api/v1/me/provider", { provider })
      .then((r) => r.data.provider),
};
