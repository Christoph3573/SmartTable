import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi } from "../../api/school";
import { schoolConnectApi } from "../../api/schoolconnect";
import {
  coursesApi,
  smartTableCourseKey,
  type Course,
  type CourseLesson,
  type CourseProvider,
  type CreateCourseInput,
} from "../../api/courses";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";

const DAYS = [
  { value: 1, label: "Montag" },
  { value: 2, label: "Dienstag" },
  { value: 3, label: "Mittwoch" },
  { value: 4, label: "Donnerstag" },
  { value: 5, label: "Freitag" },
];

type AvailableCourse = {
  provider: Exclude<CourseProvider, "custom">;
  external_key: string;
  name: string;
  short?: string;
};

function mondayOf(d: Date): Date {
  const date = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const offset = (date.getDay() + 6) % 7;
  date.setDate(date.getDate() - offset);
  return date;
}

/**
 * Kurse-Reiter (Schule): Kurse aus SmartTable und SchoolConnect mit
 * Checkboxen auswählen. Nur angehakte Kurse erscheinen im Stunden- und
 * Vertretungsplan. Eigene Kurse lassen sich mit Schulstunden anlegen.
 */
export function KursePage() {
  const client = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Course | null>(null);

  const stored = useQuery({ queryKey: ["courses"], queryFn: coursesApi.list });
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects });
  const weekOf = useMemo(() => mondayOf(new Date()).toISOString().slice(0, 10), []);

  // SmartTable: alle Fächer, die in den Stundenplänen der eigenen Klassen vorkommen.
  const smartTableAvailable = useQuery({
    queryKey: ["courses", "smarttable", classes.data?.map((c) => c.id).join(",")],
    enabled: Boolean(classes.data?.length) && Boolean(subjects.data),
    queryFn: async (): Promise<AvailableCourse[]> => {
      const subjectById = new Map((subjects.data ?? []).map((s) => [s.id, s]));
      const seen = new Map<number, AvailableCourse>();
      for (const schoolClass of classes.data ?? []) {
        const entries = await schoolApi.timetable(schoolClass.id, weekOf);
        for (const entry of entries) {
          const subject = subjectById.get(entry.lesson.subject_id);
          if (!subject || seen.has(subject.id)) continue;
          seen.set(subject.id, {
            provider: "smarttable",
            external_key: smartTableCourseKey(subject.id),
            name: subject.name,
            short: subject.short,
          });
        }
      }
      return [...seen.values()].sort((a, b) => a.name.localeCompare(b.name));
    },
  });

  // SchoolConnect: Kurskürzel aus dem Schülerportal-Stundenplan (optional).
  const external = useQuery({
    queryKey: ["courses", "schoolconnect"],
    queryFn: () => schoolConnectApi.stundenplan(),
    retry: 0,
  });
  const schoolConnectAvailable = useMemo<AvailableCourse[]>(() => {
    const seen = new Set<string>();
    const out: AvailableCourse[] = [];
    for (const entry of external.data?.eintraege ?? []) {
      const kurs = (entry.kurs || entry.tag || "").trim();
      if (!kurs || seen.has(kurs)) continue;
      seen.add(kurs);
      out.push({ provider: "schoolconnect", external_key: kurs, name: kurs, short: kurs });
    }
    return out.sort((a, b) => a.name.localeCompare(b.name));
  }, [external.data]);

  const storedByKey = useMemo(() => {
    const map = new Map<string, Course>();
    for (const course of stored.data ?? []) {
      if (course.provider !== "custom") {
        map.set(`${course.provider}:${course.external_key}`, course);
      }
    }
    return map;
  }, [stored.data]);

  const customCourses = (stored.data ?? []).filter((c) => c.provider === "custom");

  const selection = useMutation({
    mutationFn: coursesApi.select,
    onSuccess: () => client.invalidateQueries({ queryKey: ["courses"] }),
  });
  const remove = useMutation({
    mutationFn: coursesApi.remove,
    onSuccess: () => client.invalidateQueries({ queryKey: ["courses"] }),
  });

  const isSelected = (course: AvailableCourse) =>
    storedByKey.get(`${course.provider}:${course.external_key}`)?.selected ?? true;

  const toggle = (course: AvailableCourse, selected: boolean) =>
    selection.mutate({
      provider: course.provider,
      external_key: course.external_key,
      name: course.name,
      short: course.short,
      selected,
    });

  const loading = stored.isLoading || classes.isLoading || subjects.isLoading;
  const error = stored.isError;

  return (
    <div className="page">
      <PageHeader eyebrow="Schule · Kurse" title="Meine Kurse">
        <Button onClick={() => { setEditing(null); setCreateOpen(true); }}>Neuen Kurs erstellen</Button>
      </PageHeader>
      <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
        Hake ab, welche Kurse du belegst. Nur angehakte Kurse erscheinen im Stunden- und
        Vertretungsplan. Eigene Kurse kannst du mit Schulstunden anlegen.
      </p>

      {loading ? (
        <LoadingState label="Kurse werden geladen" />
      ) : error ? (
        <ErrorState onRetry={() => stored.refetch()} />
      ) : (
        <div className="flex flex-col gap-4">
          <CourseGroup
            title="SmartTable"
            description="Fächer aus den Stundenplänen deiner Klassen."
            courses={smartTableAvailable.data ?? []}
            isSelected={isSelected}
            onToggle={toggle}
            loading={smartTableAvailable.isLoading}
          />
          <CourseGroup
            title="SchoolConnect"
            description="Kurskürzel aus dem Schülerportal."
            courses={external.isError ? [] : schoolConnectAvailable}
            isSelected={isSelected}
            onToggle={toggle}
            loading={external.isLoading}
            emptyHint={
              external.isError
                ? "Nicht mit dem Schülerportal verbunden — in den Einstellungen anmelden."
                : "Das Schülerportal meldet keine Kurse."
            }
          />

          <section className="surface overflow-hidden">
            <div className="flex items-center justify-between border-b border-gray-100 px-5 py-4">
              <div>
                <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Eigene Kurse</h2>
                <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  Selbst angelegte Kurse mit eigenen Schulstunden.
                </p>
              </div>
              <Button size="sm" variant="secondary" onClick={() => { setEditing(null); setCreateOpen(true); }}>
                Hinzufügen
              </Button>
            </div>
            {customCourses.length ? (
              customCourses.map((course) => (
                <div className="data-row" key={course.id}>
                  <span
                    className="size-3.5 shrink-0 rounded-full"
                    style={{ backgroundColor: course.color || "#6366f1" }}
                    aria-hidden="true"
                  />
                  <div className="data-row-main">
                    <strong>{course.name}{course.short ? ` (${course.short})` : ""}</strong>
                    <p>{describeLessons(course.lessons)}</p>
                  </div>
                  <button className="text-button" onClick={() => { setEditing(course); setCreateOpen(true); }}>
                    Bearbeiten
                  </button>
                  <button className="text-button text-red-600" onClick={() => remove.mutate(course.id)}>
                    Löschen
                  </button>
                </div>
              ))
            ) : (
              <EmptyState
                title="Noch keine eigenen Kurse"
                description="Lege einen Kurs an und ordne ihm Schulstunden zu."
              />
            )}
          </section>
        </div>
      )}

      {createOpen && (
        <CourseDialog
          course={editing}
          onClose={() => { setCreateOpen(false); setEditing(null); }}
          onCreated={() => { setCreateOpen(false); setEditing(null); client.invalidateQueries({ queryKey: ["courses"] }); }}
        />
      )}
    </div>
  );
}

function describeLessons(lessons: CourseLesson[]): string {
  if (!lessons.length) return "Keine Schulstunden zugeordnet.";
  return lessons
    .slice()
    .sort((a, b) => a.day_of_week - b.day_of_week || a.period - b.period)
    .map((l) => `${DAYS.find((d) => d.value === l.day_of_week)?.label?.slice(0, 2) ?? "?"} ${l.period}.${l.room ? ` ${l.room}` : ""}`)
    .join(" · ");
}

function CourseGroup({
  title,
  description,
  courses,
  isSelected,
  onToggle,
  loading,
  emptyHint,
}: {
  title: string;
  description: string;
  courses: AvailableCourse[];
  isSelected: (course: AvailableCourse) => boolean;
  onToggle: (course: AvailableCourse, selected: boolean) => void;
  loading: boolean;
  emptyHint?: string;
}) {
  return (
    <section className="surface overflow-hidden">
      <div className="border-b border-gray-100 px-5 py-4">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">{title}</h2>
        <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{description}</p>
      </div>
      {loading ? (
        <div className="px-5 py-6"><LoadingState label="Kurse werden geladen" /></div>
      ) : courses.length ? (
        courses.map((course) => {
          const selected = isSelected(course);
          return (
            <label key={`${course.provider}:${course.external_key}`} className="data-row cursor-pointer">
              <input
                type="checkbox"
                className="size-4 accent-indigo-600"
                checked={selected}
                onChange={(event) => onToggle(course, event.target.checked)}
              />
              <div className="data-row-main">
                <strong>{course.name}{course.short ? ` (${course.short})` : ""}</strong>
              </div>
              <span className={`pill ${selected ? "blue" : ""}`}>{selected ? "Belegt" : "Ausgeblendet"}</span>
            </label>
          );
        })
      ) : (
        <div className="px-5 py-6 text-sm text-gray-500 dark:text-gray-400">
          {emptyHint ?? "Keine Kurse gefunden."}
        </div>
      )}
    </section>
  );
}

function CourseDialog({
  course,
  onClose,
  onCreated,
}: {
  course: Course | null;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState(course?.name ?? "");
  const [short, setShort] = useState(course?.short ?? "");
  const [color, setColor] = useState(course?.color ?? "#6366f1");
  const [lessons, setLessons] = useState<CourseLesson[]>(course?.lessons ?? [{ day_of_week: 1, period: 1, room: "" }]);
  const [error, setError] = useState(false);

  const save = useMutation({
    mutationFn: async () => {
      const cleaned = lessons.filter((l) => l.period >= 1);
      if (course) {
        await coursesApi.update(course.id, { name, short, color, lessons: cleaned });
      } else {
        const data: CreateCourseInput = { name, short: short || undefined, color, lessons: cleaned };
        await coursesApi.create(data);
      }
    },
    onSuccess: onCreated,
    onError: () => setError(true),
  });

  const setLesson = (index: number, patch: Partial<CourseLesson>) =>
    setLessons((prev) => prev.map((l, i) => (i === index ? { ...l, ...patch } : l)));

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <form
        className="modal"
        onSubmit={(event) => { event.preventDefault(); if (name.trim()) save.mutate(); }}
      >
        <h2>{course ? "Kurs bearbeiten" : "Neuen Kurs erstellen"}</h2>
        <div className="form-grid">
          <label className="full">
            Name
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="z. B. Französisch"
            />
          </label>
          <label>
            Kürzel
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 p-2 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              value={short}
              onChange={(event) => setShort(event.target.value)}
              placeholder="z. B. F"
            />
          </label>
          <label>
            Farbe
            <input
              className="mt-1 h-10 w-full rounded-lg border border-gray-300 p-1 dark:border-gray-600 dark:bg-gray-800"
              type="color"
              value={color}
              onChange={(event) => setColor(event.target.value)}
            />
          </label>
        </div>

        <fieldset className="mt-4">
          <legend className="text-sm font-medium text-gray-700 dark:text-gray-200">Schulstunden</legend>
          <div className="mt-2 flex flex-col gap-2">
            {lessons.map((lesson, index) => (
              <div key={index} className="flex items-end gap-2">
                <label className="flex-1 text-xs text-gray-500 dark:text-gray-400">
                  Tag
                  <select
                    className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    value={lesson.day_of_week}
                    onChange={(event) => setLesson(index, { day_of_week: Number(event.target.value) })}
                  >
                    {DAYS.map((day) => <option key={day.value} value={day.value}>{day.label}</option>)}
                  </select>
                </label>
                <label className="w-20 text-xs text-gray-500 dark:text-gray-400">
                  Stunde
                  <input
                    className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    type="number"
                    min="1"
                    max="12"
                    value={lesson.period}
                    onChange={(event) => setLesson(index, { period: Number(event.target.value) })}
                  />
                </label>
                <label className="w-24 text-xs text-gray-500 dark:text-gray-400">
                  Raum
                  <input
                    className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    value={lesson.room ?? ""}
                    onChange={(event) => setLesson(index, { room: event.target.value })}
                  />
                </label>
                <button
                  type="button"
                  className="pb-2 text-sm text-red-600 hover:text-red-800"
                  aria-label="Stunde entfernen"
                  onClick={() => setLessons((prev) => prev.filter((_, i) => i !== index))}
                >
                  ×
                </button>
              </div>
            ))}
          </div>
          <button
            type="button"
            className="mt-2 text-sm font-medium text-indigo-700 hover:text-indigo-900 dark:text-indigo-300"
            onClick={() => setLessons((prev) => [...prev, { day_of_week: 1, period: prev.length + 1, room: "" }])}
          >
            + Stunde hinzufügen
          </button>
        </fieldset>

        {error && <p className="mt-3 text-sm text-red-600">Der Kurs konnte nicht gespeichert werden.</p>}
        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button>
          <Button type="submit" loading={save.isPending}>{course ? "Speichern" : "Anlegen"}</Button>
        </div>
      </form>
    </div>
  );
}
