import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { schoolApi, type LessonInput, type TimetableEntry, type UpdateLessonInput } from "../../api/school";
import { schoolConnectApi, scTimetableEntryLabel, type SchuelerportalTimetableEntry } from "../../api/schoolconnect";
import { useAuthStore } from "../../store/authStore";
import { useSettingsStore } from "../../store/settingsStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";

const DAYS = [
  { value: 1, label: "Montag" },
  { value: 2, label: "Dienstag" },
  { value: 3, label: "Mittwoch" },
  { value: 4, label: "Donnerstag" },
  { value: 5, label: "Freitag" },
];
const PERIODS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

// Monday (ISO week start) of the week containing `d`.
function mondayOf(d: Date): Date {
  const date = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const offset = (date.getDay() + 6) % 7; // Sunday=0 -> 6, Monday=1 -> 0, ...
  date.setDate(date.getDate() - offset);
  return date;
}
function toISODate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

export function TimetablePage() {
  const user = useAuthStore((s) => s.user);
  const client = useQueryClient();
  const [classId, setClassId] = useState<number>();
  const [weekAnchor, setWeekAnchor] = useState(() => mondayOf(new Date()));
  const [editing, setEditing] = useState<{ dayOfWeek: number; period: number; entry?: TimetableEntry } | null>(null);

  const provider = useSettingsStore((s) => s.provider);
  const isExternal = provider === "schoolconnect";
  const canManage = !isExternal && (user?.role === "teacher" || user?.role === "school_admin" || user?.role === "superadmin" || user?.role === "admin");

  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes, enabled: !isExternal });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects, enabled: !isExternal });
  const activeClassId = classId ?? classes.data?.[0]?.id;
  const weekOf = toISODate(weekAnchor);
  const timetable = useQuery({
    queryKey: ["timetable", activeClassId, weekOf],
    queryFn: () => schoolApi.timetable(activeClassId!, weekOf),
    enabled: !isExternal && Boolean(activeClassId),
  });
  const external = useQuery({
    queryKey: ["schoolconnect", "stundenplan"],
    queryFn: () => schoolConnectApi.stundenplan(),
    enabled: isExternal,
    retry: 1,
  });

  const createLesson = useMutation({
    mutationFn: (data: LessonInput) => schoolApi.createLesson(activeClassId!, data),
    onSuccess: () => { client.invalidateQueries({ queryKey: ["timetable", activeClassId] }); setEditing(null); },
  });
  const updateLesson = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateLessonInput }) => schoolApi.updateLesson(id, data),
    onSuccess: () => { client.invalidateQueries({ queryKey: ["timetable", activeClassId] }); setEditing(null); },
  });
  const deleteLesson = useMutation({
    mutationFn: (id: number) => schoolApi.deleteLesson(id),
    onSuccess: () => client.invalidateQueries({ queryKey: ["timetable", activeClassId] }),
  });

  const byCell = useMemo(() => {
    const map = new Map<string, TimetableEntry>();
    (timetable.data ?? []).forEach((entry) => map.set(`${entry.lesson.day_of_week}-${entry.lesson.period}`, entry));
    return map;
  }, [timetable.data]);

  const externalByCell = useMemo(() => {
    const map = new Map<string, SchuelerportalTimetableEntry>();
    (external.data?.eintraege ?? []).forEach((entry) =>
      map.set(`${entry.day + 1}-${entry.stunde}`, entry)
    );
    return map;
  }, [external.data]);

  const subjectShort = (id: number) => subjects.data?.find((s) => s.id === id)?.short ?? "?";

  const weekLabel = `${weekAnchor.toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit" })} – ${new Date(weekAnchor.getTime() + 4 * 86400000).toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit" })}`;

  if (isExternal) {
    return (
      <div className="page">
        <PageHeader eyebrow="Wochenplan · SchoolConnect" title="Stundenplan" />
        {external.isLoading ? (
          <LoadingState label="Stundenplan wird geladen" />
        ) : external.isError ? (
          axios.isAxiosError(external.error) && external.error.response?.status === 401 ? (
            <div className="empty-state">
              <span aria-hidden="true">✦</span>
              <strong>Nicht beim Schülerportal angemeldet</strong>
              <p>Bitte in den <Link className="font-semibold text-indigo-700 hover:underline" to="/settings">Einstellungen → SchoolConnect</Link> anmelden.</p>
              <Button size="sm" variant="secondary" onClick={() => external.refetch()}>Erneut versuchen</Button>
            </div>
          ) : (
            <ErrorState onRetry={() => external.refetch()} message="Der Stundenplan konnte von SchoolConnect nicht geladen werden." />
          )
        ) : !external.data?.eintraege.length ? (
          <EmptyState title="Keine Stunden gefunden" description="Das Schülerportal meldet keine Stundenplaneinträge (ggf. via Kurskürzel-Filter prüfen)." />
        ) : (
          <>
            <p className="mb-3 text-xs text-gray-500 dark:text-gray-400">
              Quelle: Schülerportal via SchoolConnect — schreibgeschützt, alle Kurse der Stufe (ggf. via Kurskürzel filtern).
            </p>
            <section className="overflow-x-auto rounded-xl border border-gray-200 bg-white">
              <table className="w-full min-w-[720px] border-collapse text-sm">
                <thead>
                  <tr>
                    <th className="w-16 border-b border-gray-100 bg-gray-50 px-2 py-2 text-left text-xs font-semibold text-gray-500">Std.</th>
                    {DAYS.map((day) => (
                      <th key={day.value} className="border-b border-gray-100 bg-gray-50 px-2 py-2 text-left text-xs font-semibold text-gray-500">{day.label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {PERIODS.map((period) => (
                    <tr key={period}>
                      <td className="border-b border-gray-50 px-2 py-2 text-center font-mono text-xs text-gray-400">{period}.</td>
                      {DAYS.map((day) => {
                        const entry = externalByCell.get(`${day.value}-${period}`);
                        if (!entry) return <td key={day.value} className="border-b border-gray-50 px-1.5 py-1.5 align-top"><div className="h-14" /></td>;
                        const label = scTimetableEntryLabel(entry);
                        return (
                          <td key={day.value} className="border-b border-gray-50 px-1.5 py-1.5 align-top">
                            <div className="flex h-14 flex-col justify-center rounded-lg border border-indigo-100 bg-indigo-50 px-2 py-1 text-xs text-indigo-800">
                              <strong className="font-semibold">{label.subject}</strong>
                              <span className="text-[11px] opacity-80">{label.room}</span>
                            </div>
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          </>
        )}
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader eyebrow="Wochenplan" title="Stundenplan">
        <select
          aria-label="Klasse wählen"
          className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm"
          value={activeClassId ?? ""}
          onChange={(event) => setClassId(Number(event.target.value))}
        >
          {classes.data?.map((schoolClass) => (
            <option key={schoolClass.id} value={schoolClass.id}>{schoolClass.name}</option>
          ))}
        </select>
        <div className="flex items-center gap-2">
          <button className="text-button" onClick={() => setWeekAnchor((prev) => new Date(prev.getTime() - 7 * 86400000))}>◀</button>
          <span className="text-sm font-medium text-gray-700">{weekLabel}</span>
          <button className="text-button" onClick={() => setWeekAnchor((prev) => new Date(prev.getTime() + 7 * 86400000))}>▶</button>
          <button className="text-button" onClick={() => setWeekAnchor(mondayOf(new Date()))}>Heute</button>
        </div>
      </PageHeader>

      {classes.isLoading || (activeClassId && timetable.isLoading) ? (
        <LoadingState />
      ) : classes.isError || timetable.isError ? (
        <ErrorState onRetry={() => { classes.refetch(); timetable.refetch(); }} />
      ) : !activeClassId ? (
        <EmptyState title="Keine Klasse verfügbar" description="Sobald du einer Klasse zugeordnet bist, findest du hier ihren Stundenplan." />
      ) : (
        <section className="overflow-x-auto rounded-xl border border-gray-200 bg-white">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              <tr>
                <th className="w-16 border-b border-gray-100 bg-gray-50 px-2 py-2 text-left text-xs font-semibold text-gray-500">Std.</th>
                {DAYS.map((day) => (
                  <th key={day.value} className="border-b border-gray-100 bg-gray-50 px-2 py-2 text-left text-xs font-semibold text-gray-500">{day.label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {PERIODS.map((period) => (
                <tr key={period}>
                  <td className="border-b border-gray-50 px-2 py-2 text-center font-mono text-xs text-gray-400">{period}.</td>
                  {DAYS.map((day) => {
                    const entry = byCell.get(`${day.value}-${period}`);
                    return (
                      <td key={day.value} className="border-b border-gray-50 px-1.5 py-1.5 align-top">
                        {entry ? (
                          <TimetableCell
                            entry={entry}
                            subjectShort={subjectShort}
                            canManage={canManage}
                            onEdit={() => setEditing({ dayOfWeek: day.value, period, entry })}
                            onDelete={() => deleteLesson.mutate(entry.lesson.id)}
                          />
                        ) : canManage ? (
                          <button
                            className="flex h-14 w-full items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-300 hover:border-indigo-300 hover:text-indigo-500"
                            onClick={() => setEditing({ dayOfWeek: day.value, period })}
                          >
                            +
                          </button>
                        ) : (
                          <div className="h-14" />
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      {editing && (
        <LessonDialog
          dayOfWeek={editing.dayOfWeek}
          period={editing.period}
          lesson={editing.entry?.lesson}
          subjects={subjects.data ?? []}
          currentUserId={user!.id}
          pending={createLesson.isPending || updateLesson.isPending}
          error={createLesson.isError || updateLesson.isError}
          onClose={() => setEditing(null)}
          onSubmit={(data) => {
            if (editing.entry) updateLesson.mutate({ id: editing.entry.lesson.id, data });
            else createLesson.mutate({ ...data, day_of_week: editing.dayOfWeek, period: editing.period } as LessonInput);
          }}
        />
      )}
    </div>
  );
}

function TimetableCell({
  entry,
  subjectShort,
  canManage,
  onEdit,
  onDelete,
}: {
  entry: TimetableEntry;
  subjectShort: (id: number) => string;
  canManage: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const cancelled = entry.type === "cancelled";
  const substituted = entry.type === "substituted";
  return (
    <div
      className={`group relative flex h-14 flex-col justify-center rounded-lg border px-2 py-1 text-xs ${
        cancelled
          ? "border-red-200 bg-red-50 text-red-700 line-through decoration-red-400"
          : substituted
          ? "border-amber-200 bg-amber-50 text-amber-800"
          : "border-indigo-100 bg-indigo-50 text-indigo-800"
      }`}
    >
      <strong className="font-semibold">{subjectShort(entry.lesson.subject_id)}</strong>
      <span className="text-[11px] opacity-80">{entry.lesson.room ? `Raum ${entry.lesson.room}` : "—"}</span>
      {cancelled && <span className="text-[11px] font-medium">Entfällt</span>}
      {substituted && <span className="text-[11px] font-medium">Vertretung</span>}
      {canManage && (
        <div className="absolute inset-x-1 bottom-1 hidden justify-end gap-1 group-hover:flex">
          <button className="rounded bg-white/80 px-1 text-[10px] font-medium text-gray-600 hover:bg-white" onClick={onEdit}>Bearb.</button>
          <button className="rounded bg-white/80 px-1 text-[10px] font-medium text-red-600 hover:bg-white" onClick={onDelete}>Löschen</button>
        </div>
      )}
    </div>
  );
}

function LessonDialog({
  dayOfWeek,
  period,
  lesson,
  subjects,
  currentUserId,
  onClose,
  onSubmit,
  pending,
  error,
}: {
  dayOfWeek: number;
  period: number;
  lesson?: TimetableEntry["lesson"];
  subjects: { id: number; name: string; short: string }[];
  currentUserId: number;
  onClose: () => void;
  onSubmit: (data: LessonInput | UpdateLessonInput) => void;
  pending: boolean;
  error: boolean;
}) {
  const dayLabel = DAYS.find((d) => d.value === dayOfWeek)?.label ?? "";
  const [form, setForm] = useState({
    subject_id: String(lesson?.subject_id ?? ""),
    room: lesson?.room ?? "",
  });

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <form
        className="modal"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit({
            subject_id: Number(form.subject_id),
            teacher_id: lesson?.teacher_id ?? currentUserId,
            room: form.room || undefined,
          });
        }}
      >
        <h2>{lesson ? "Stunde bearbeiten" : "Stunde anlegen"}</h2>
        <p className="mb-3 text-sm text-gray-500">{dayLabel}, {period}. Stunde</p>
        <div className="form-grid">
          <label className="full">
            Fach
            <select
              className="mt-1 w-full rounded-lg border border-gray-300 p-2"
              required
              value={form.subject_id}
              onChange={(event) => setForm({ ...form, subject_id: event.target.value })}
            >
              <option value="">Auswählen</option>
              {subjects.map((subject) => (
                <option key={subject.id} value={subject.id}>{subject.name}</option>
              ))}
            </select>
          </label>
          <label className="full">
            Raum
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2"
              value={form.room}
              onChange={(event) => setForm({ ...form, room: event.target.value })}
            />
          </label>
        </div>
        {error && <p className="mt-3 text-sm text-red-600">Die Stunde konnte nicht gespeichert werden.</p>}
        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button>
          <Button type="submit" loading={pending}>{lesson ? "Änderungen speichern" : "Anlegen"}</Button>
        </div>
      </form>
    </div>
  );
}
