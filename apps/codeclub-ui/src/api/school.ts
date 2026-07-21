import { apiClient } from "./client";
import type { components } from "./generated/types";

export type SchoolClass = components["schemas"]["Class"];
export type SchoolUser = components["schemas"]["User"];
export type CreateUserInput = components["schemas"]["CreateUserRequest"];
export type UpdateUserInput = components["schemas"]["UpdateUserRequest"];
export type ClassMember = components["schemas"]["ClassMember"];
export type ClassTeacher = components["schemas"]["ClassTeacher"];
export type Subject = components["schemas"]["Subject"];
export type Substitution = components["schemas"]["Substitution"];
export type CalendarEvent = components["schemas"]["Event"];
export type SchoolFile = components["schemas"]["File"];
export type FileFolder = components["schemas"]["FileFolder"];
export type Homework = components["schemas"]["Homework"];
export type HomeworkSubmission = components["schemas"]["HomeworkSubmission"];
export type ChatChannel = components["schemas"]["ChatChannel"];
export type ChatMessage = components["schemas"]["Message"];

export type EventInput = components["schemas"]["CreateEventRequest"];
export type HomeworkInput = components["schemas"]["CreateHomeworkRequest"];
export type SubstitutionInput = components["schemas"]["CreateSubstitutionRequest"];

export const schoolApi = {
  users: () => apiClient.get<SchoolUser[]>("/api/v1/users").then((r) => r.data),
  createUser: (data: CreateUserInput) =>
    apiClient.post<SchoolUser>("/api/v1/users", data).then((r) => r.data),
  createUsersBulk: (users: CreateUserInput[]) =>
    apiClient.post<{ users: SchoolUser[] }>("/api/v1/users/bulk", { users }).then((r) => r.data.users),
  updateUser: (id: number, data: UpdateUserInput) =>
    apiClient.patch<SchoolUser>(`/api/v1/users/${id}`, data).then((r) => r.data),
  deleteUser: (id: number) => apiClient.delete(`/api/v1/users/${id}`),

  classes: () => apiClient.get<SchoolClass[]>("/api/v1/classes").then((r) => r.data),
  createClass: (data: Pick<SchoolClass, "name" | "school_year">) =>
    apiClient.post<SchoolClass>("/api/v1/classes", data).then((r) => r.data),
  updateClass: (id: number, data: Partial<Pick<SchoolClass, "name" | "school_year">>) =>
    apiClient.patch<SchoolClass>(`/api/v1/classes/${id}`, data).then((r) => r.data),
  deleteClass: (id: number) => apiClient.delete(`/api/v1/classes/${id}`),
  classMembers: (id: number) =>
    apiClient.get<ClassMember[]>(`/api/v1/classes/${id}/members`).then((r) => r.data),
  addClassMember: (classId: number, userId: number) =>
    apiClient.post(`/api/v1/classes/${classId}/members`, { user_id: userId }),
  removeClassMember: (classId: number, userId: number) =>
    apiClient.delete(`/api/v1/classes/${classId}/members/${userId}`),
  classTeachers: (id: number) =>
    apiClient.get<ClassTeacher[]>(`/api/v1/classes/${id}/teachers`).then((r) => r.data),
  addClassTeacher: (classId: number, userId: number, isHomeTeacher = false) =>
    apiClient.post(`/api/v1/classes/${classId}/teachers`, { user_id: userId, is_home_teacher: isHomeTeacher }),
  removeClassTeacher: (classId: number, userId: number) =>
    apiClient.delete(`/api/v1/classes/${classId}/teachers/${userId}`),

  subjects: () => apiClient.get<Subject[]>("/api/v1/subjects").then((r) => r.data),
  substitutions: (params?: { date_from?: string; date_to?: string; class_id?: number }) =>
    apiClient.get<Substitution[]>("/api/v1/substitutions", { params }).then((r) => r.data),
  createSubstitution: (data: SubstitutionInput) =>
    apiClient.post<Substitution>("/api/v1/substitutions", data).then((r) => r.data),
  deleteSubstitution: (id: number) => apiClient.delete(`/api/v1/substitutions/${id}`),

  events: (params?: { start_date?: string; end_date?: string; class_id?: number }) =>
    apiClient.get<CalendarEvent[]>("/api/v1/events", { params }).then((r) => r.data),
  createEvent: (data: EventInput) => apiClient.post<CalendarEvent>("/api/v1/events", data).then((r) => r.data),
  deleteEvent: (id: number) => apiClient.delete(`/api/v1/events/${id}`),

  homework: (classId: number) =>
    apiClient.get<Homework[]>(`/api/v1/classes/${classId}/homework`).then((r) => r.data),
  createHomework: (classId: number, data: HomeworkInput) =>
    apiClient.post<Homework>(`/api/v1/classes/${classId}/homework`, data).then((r) => r.data),
  deleteHomework: (id: number) => apiClient.delete(`/api/v1/homework/${id}`),
  submissions: (homeworkId: number) =>
    apiClient.get<HomeworkSubmission[]>(`/api/v1/homework/${homeworkId}/submissions`).then((r) => r.data),
  submitHomework: (homeworkId: number) =>
    apiClient.post<HomeworkSubmission>(`/api/v1/homework/${homeworkId}/submissions`).then((r) => r.data),

  folders: (classId: number) =>
    apiClient.get<FileFolder[]>(`/api/v1/classes/${classId}/folders`).then((r) => r.data),
  files: (classId: number, folderId?: number) =>
    apiClient.get<SchoolFile[]>(`/api/v1/classes/${classId}/files`, { params: { folder_id: folderId } }).then((r) => r.data),
  uploadFile: (classId: number, file: File, folderId?: number) => {
    const data = new FormData();
    data.set("file", file);
    if (folderId) data.set("folder_id", String(folderId));
    return apiClient.post<SchoolFile>(`/api/v1/classes/${classId}/files`, data).then((r) => r.data);
  },
  downloadFile: (id: number) => apiClient.get(`/api/v1/files/${id}`, { responseType: "blob" }),
  deleteFile: (id: number) => apiClient.delete(`/api/v1/files/${id}`),

  channels: () => apiClient.get<ChatChannel[]>("/api/v1/channels").then((r) => r.data),
  messages: (channelId: number) =>
    apiClient.get<ChatMessage[]>(`/api/v1/channels/${channelId}/messages`, { params: { limit: 50 } }).then((r) => r.data),
  sendMessage: (channelId: number, content: string) =>
    apiClient.post<ChatMessage>(`/api/v1/channels/${channelId}/messages`, { content }).then((r) => r.data),
};
