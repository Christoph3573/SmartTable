import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { schoolApi } from "../../api/school";
import { schoolConnectApi } from "../../api/schoolconnect";
import { useCourseVisibility } from "../../lib/useCourseVisibility";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";

const SCHULARTEN = ["Gymnasium", "Realschule", "Mittelschule", "FOSBOS", "Grundschule"];
const LEHRPLANKAPITEL = [
  { value: "kap1", label: "Kapitel 1" },
  { value: "kap2", label: "Kapitel 2" },
  { value: "kap3", label: "Kapitel 3" },
  { value: "kap4", label: "Kapitel 4" },
  { value: "kap5", label: "Kapitel 5" },
];

function field(item: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = item[key];
    if (typeof value === "string" && value.trim()) return value;
    if (typeof value === "number") return String(value);
  }
  return "";
}

/**
 * Lernplan: eigene Kurse (aus dem Stundenplan abgeleitete Fächer der
 * Klasse) plus offizielle LehrplanPLUS-Inhalte via lernplan-bayern
 * (SchoolConnect, kein Login nötig, tenantlos).
 */
export function StudyPlanPage() {
  const [classId, setClassId] = useState<number>();
  const [schulart, setSchulart] = useState("Gymnasium");
  const [kapitel, setKapitel] = useState("kap4");
  const [fach, setFach] = useState("");
  const [jahrgangsstufe, setJahrgangsstufe] = useState("");
  const [search, setSearch] = useState<Record<string, string> | null>(null);

  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects });
  const visibility = useCourseVisibility();
  const activeClassId = classId ?? classes.data?.[0]?.id;
  const activeClass = classes.data?.find((c) => c.id === activeClassId);
  const timetable = useQuery({
    queryKey: ["timetable", activeClassId],
    queryFn: () => schoolApi.timetable(activeClassId!),
    enabled: Boolean(activeClassId),
  });

  const mySubjects = useMemo(() => {
    const ids = new Set(
      (timetable.data ?? [])
        .filter((entry) => !visibility.hiddenSubject(entry.lesson.subject_id))
        .map((entry) => entry.lesson.subject_id)
    );
    return (subjects.data ?? []).filter((s) => ids.has(s.id));
  }, [timetable.data, subjects.data, visibility]);

  const lehrplan = useQuery({
    queryKey: ["lernplan-bayern", "search", search],
    queryFn: () =>
      schoolConnectApi.call<{ treffer?: Record<string, unknown>[]; results?: Record<string, unknown>[] }>(
        "lernplan-bayern",
        "search",
        search ?? undefined
      ),
    enabled: search !== null,
    retry: 1,
  });

  const submitSearch = (event: React.FormEvent) => {
    event.preventDefault();
    const params: Record<string, string> = { schulart, lehrplankapitel: kapitel };
    if (fach.trim()) params.fach = fach.trim();
    if (jahrgangsstufe.trim()) params.jahrgangsstufe = jahrgangsstufe.trim();
    setSearch(params);
  };

  return (
    <div className="page">
      <PageHeader eyebrow="Lern-Bereich" title="Lernplan">
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
      </PageHeader>

      <div className="grid gap-5 lg:grid-cols-[1fr_1.2fr]">
        <section className="surface overflow-hidden">
          <div className="border-b border-gray-100 px-5 py-4">
            <h2>Meine Kurse{activeClass ? ` · ${activeClass.name}` : ""}</h2>
            <p className="mt-1 text-xs text-gray-500">Fächer aus deinem Stundenplan dieser Klasse.</p>
          </div>
          {timetable.isLoading || subjects.isLoading ? (
            <LoadingState label="Kurse werden geladen" />
          ) : timetable.isError || subjects.isError ? (
            <ErrorState onRetry={() => { timetable.refetch(); subjects.refetch(); }} />
          ) : !mySubjects.length ? (
            <EmptyState title="Keine Kurse gefunden" description="Für diese Klasse ist noch kein Stundenplan hinterlegt." />
          ) : (
            mySubjects.map((subject) => (
              <div className="data-row" key={subject.id}>
                <span className="grid size-11 place-items-center rounded-lg bg-indigo-50 text-center font-mono text-xs text-indigo-700">
                  {subject.short}
                </span>
                <div className="data-row-main">
                  <strong>{subject.name}</strong>
                  <p>
                    {(timetable.data ?? []).filter((e) => e.lesson.subject_id === subject.id).length}x pro Woche
                    · Stundenplan ansehen
                  </p>
                </div>
                <button
                  className="text-button"
                  onClick={() => setFach(subject.name)}
                  title="Fach in die Lehrplan-Suche übernehmen"
                >
                  Lehrplan →
                </button>
              </div>
            ))
          )}
        </section>

        <section className="surface overflow-hidden">
          <div className="border-b border-gray-100 px-5 py-4">
            <h2>LehrplanPLUS (Bayern)</h2>
            <p className="mt-1 text-xs text-gray-500">Offizielle Inhalte via SchoolConnect · kein Login nötig.</p>
          </div>
          <form onSubmit={submitSearch} className="grid gap-2 border-b border-gray-100 p-5 sm:grid-cols-2">
            <label className="text-xs text-gray-600">
              Schulart
              <select className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm" value={schulart} onChange={(e) => setSchulart(e.target.value)}>
                {SCHULARTEN.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </label>
            <label className="text-xs text-gray-600">
              Lehrplankapitel
              <select className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm" value={kapitel} onChange={(e) => setKapitel(e.target.value)}>
                {LEHRPLANKAPITEL.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
              </select>
            </label>
            <label className="text-xs text-gray-600">
              Fach
              <input className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm" value={fach} onChange={(e) => setFach(e.target.value)} placeholder="z. B. Deutsch" />
            </label>
            <label className="text-xs text-gray-600">
              Jahrgangsstufe
              <input className="mt-1 w-full rounded-lg border border-gray-300 p-2 text-sm" value={jahrgangsstufe} onChange={(e) => setJahrgangsstufe(e.target.value)} placeholder="z. B. 9" />
            </label>
            <div className="flex items-end sm:col-span-2">
              <button type="submit" className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700">
                Lehrplan suchen
              </button>
            </div>
          </form>
          <div className="p-5">
            {search === null ? (
              <EmptyState title="Noch keine Suche" description="Wähle Schulart, Kapitel, Fach und Jahrgangsstufe und starte die Suche." />
            ) : lehrplan.isLoading ? (
              <LoadingState label="Lehrplan wird geladen" />
            ) : lehrplan.isError ? (
              <ErrorState onRetry={() => lehrplan.refetch()} message="LehrplanPLUS antwortet gerade nicht (SchoolConnect-Sidecar prüfen)." />
            ) : (
              <LehrplanResults data={lehrplan.data} />
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

function LehrplanResults({ data }: { data?: { treffer?: Record<string, unknown>[]; results?: Record<string, unknown>[] } & Record<string, unknown> }) {
  const items: Record<string, unknown>[] =
    data?.treffer ?? data?.results ??
    (Array.isArray(data) ? (data as Record<string, unknown>[]) : []);
  if (!items.length) {
    return <EmptyState title="Keine Treffer" description="Für diese Auswahl meldet LehrplanPLUS keine Inhalte." />;
  }
  return (
    <ul className="flex flex-col gap-2">
      {items.map((item, index) => {
        const title = field(item, "titel", "title", "thema", "name", "kompetenz");
        const detail = field(item, "beschreibung", "description", "text", "inhalt", "fach");
        return (
          <li key={index} className="rounded-xl border border-gray-200 p-3">
            <strong className="block text-sm text-gray-900">{title || `Treffer ${index + 1}`}</strong>
            {detail && <p className="mt-1 text-xs text-gray-500">{detail}</p>}
          </li>
        );
      })}
    </ul>
  );
}
