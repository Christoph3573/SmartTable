import { apiClient } from "./client";

/**
 * Kurse (Schule): Auswahl, welche Kurse der Nutzer belegt. Abgewählte Kurse
 * (selected=false) werden im Stunden-/Vertretungsplan ausgeblendet; eigene
 * Kurse (provider=custom) bringen zusätzliche Kursstunden mit.
 *
 * Die verfügbaren Kurse werden clientseitig aus SmartTable (Fächer des
 * Stundenplans) bzw. SchoolConnect (Kurskürzel) abgeleitet und hier nur als
 * Auswahl gespeichert. Fehlt eine Zeile, gilt der Kurs als sichtbar.
 */
export type CourseProvider = "smarttable" | "schoolconnect" | "custom";

export type CourseLesson = {
  day_of_week: number;
  period: number;
  room?: string;
};

export type Course = {
  id: number;
  provider: CourseProvider;
  external_key: string;
  name: string;
  short?: string;
  color?: string;
  selected: boolean;
  lessons: CourseLesson[];
};

export type CourseSelectionInput = {
  provider: Exclude<CourseProvider, "custom">;
  external_key: string;
  name: string;
  short?: string;
  color?: string;
  selected: boolean;
};

export type CreateCourseInput = {
  name: string;
  short?: string;
  color?: string;
  lessons?: CourseLesson[];
};

export type UpdateCourseInput = {
  name?: string;
  short?: string;
  color?: string;
  selected?: boolean;
  lessons?: CourseLesson[];
};

export const coursesApi = {
  list: () => apiClient.get<Course[]>("/api/v1/courses").then((r) => r.data),
  create: (data: CreateCourseInput) =>
    apiClient.post<Course>("/api/v1/courses", data).then((r) => r.data),
  select: (data: CourseSelectionInput) =>
    apiClient.put<Course>("/api/v1/courses/selection", data).then((r) => r.data),
  update: (id: number, data: UpdateCourseInput) =>
    apiClient.patch<Course>(`/api/v1/courses/${id}`, data).then((r) => r.data),
  remove: (id: number) => apiClient.delete(`/api/v1/courses/${id}`),
};

/** Stabile externe Kennung für SmartTable-Fächer. */
export function smartTableCourseKey(subjectId: number): string {
  return `subject:${subjectId}`;
}
