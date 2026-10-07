import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { schoolApi, type Homework, type HomeworkInput, type HomeworkSubmission, type UpdateHomeworkInput } from "../../api/school";
import { schoolConnectApi } from "../../api/schoolconnect";
import { useAuthStore } from "../../store/authStore";
import { useSettingsStore } from "../../store/settingsStore";
import { useCourseVisibility } from "../../lib/useCourseVisibility";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

export function HomeworkPage() {
  const [classId, setClassId] = useState<number>();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Homework>();
  const [grading, setGrading] = useState<Homework>();
  const [pendingFiles, setPendingFiles] = useState<Record<number, File | undefined>>({});
  const client = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const provider = useSettingsStore((s) => s.provider);
  const isExternal = provider === "schoolconnect";
  const visibility = useCourseVisibility();
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes, enabled: !isExternal });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects, enabled: !isExternal });
  const activeClassId = classId ?? classes.data?.[0]?.id;
  const homework = useQuery({ queryKey: ["homework", activeClassId], queryFn: () => schoolApi.homework(activeClassId!), enabled: !isExternal && Boolean(activeClassId) });
  const external = useQuery({
    queryKey: ["schoolconnect", "hausaufgaben"],
    queryFn: schoolConnectApi.hausaufgaben,
    enabled: isExternal,
    retry: 1,
  });
  const create = useMutation({ mutationFn: (data: HomeworkInput) => schoolApi.createHomework(activeClassId!, data), onSuccess: () => { client.invalidateQueries({ queryKey: ["homework", activeClassId] }); setCreateOpen(false); } });
  const update = useMutation({ mutationFn: ({ id, data }: { id: number; data: UpdateHomeworkInput }) => schoolApi.updateHomework(id, data), onSuccess: () => { client.invalidateQueries({ queryKey: ["homework", activeClassId] }); setEditing(undefined); } });
  const submit = useMutation({
    mutationFn: ({ id, file }: { id: number; file?: File }) => schoolApi.submitHomework(id, file),
    onSuccess: (_, { id }) => { client.invalidateQueries({ queryKey: ["submissions", id] }); setPendingFiles((prev) => ({ ...prev, [id]: undefined })); },
  });
  const withdraw = useMutation({
    mutationFn: ({ submissionId }: { homeworkId: number; submissionId: number }) => schoolApi.withdrawSubmission(submissionId),
    onSuccess: (_, { homeworkId }) => client.invalidateQueries({ queryKey: ["submissions", homeworkId] }),
  });
  const remove = useMutation({ mutationFn: schoolApi.deleteHomework, onSuccess: () => client.invalidateQueries({ queryKey: ["homework", activeClassId] }) });
  const isTeacher = user?.role === "teacher" || user?.role === "school_admin" || user?.role === "superadmin" || user?.role === "admin";
  const submissionQueries = useQueries({ queries: !isTeacher ? (homework.data ?? []).map((item) => ({ queryKey: ["submissions", item.id], queryFn: () => schoolApi.submissions(item.id) })) : [] });
  const ownSubmission = new Map<number, HomeworkSubmission>();
  (homework.data ?? []).forEach((item, index) => {
    const own = submissionQueries[index]?.data?.[0];
    if (own && (own.status === "submitted" || own.status === "graded")) ownSubmission.set(item.id, own);
  });
  const subject = (id: number) => subjects.data?.find((entry) => entry.id === id)?.short ?? "Fach";
  const selectClass = (id: number) => setClassId(id);
  const ownHomework = (homework.data ?? []).filter((item) => !visibility.hiddenSubject(item.subject_id));

  if (isExternal) {
    const aufgaben = (external.data?.aufgaben ?? []).filter(
      (item) => !visibility.hiddenKurs(field(item, "uf", "kurs", "fach"))
    );
    return (
      <div className="page">
        <PageHeader eyebrow="Lernplan · SchoolConnect" title="Hausaufgaben" />
        {external.isLoading ? <LoadingState label="Hausaufgaben werden geladen" /> : external.isError ? (
          axios.isAxiosError(external.error) && external.error.response?.status === 401 ? (
            <div className="empty-state">
              <span aria-hidden="true">✦</span>
              <strong>Nicht beim Schülerportal angemeldet</strong>
              <p>Bitte in den <Link className="font-semibold text-indigo-700 hover:underline" to="/settings">Einstellungen → SchoolConnect</Link> anmelden.</p>
              <Button size="sm" variant="secondary" onClick={() => external.refetch()}>Erneut versuchen</Button>
            </div>
          ) : (
            <ErrorState onRetry={() => external.refetch()} message="Die Hausaufgaben konnten von SchoolConnect nicht geladen werden." />
          )
        ) : (
          <section className="overflow-hidden rounded-xl border border-gray-200 bg-white">
            <div className="flex items-center justify-between border-b border-gray-100 px-5 py-3">
              <span className="text-sm font-bold">{aufgaben.length} Aufgabe{aufgaben.length === 1 ? "" : "n"}</span>
              <span className="text-xs text-gray-500">Quelle: Schülerportal via SchoolConnect — schreibgeschützt.</span>
            </div>
            {aufgaben.length ? aufgaben.map((item, index) => {
              const title = field(item, "titel", "title", "fach", "subject", "uf");
              const description = field(item, "beschreibung", "description", "text", "aufgabe");
              const due = field(item, "faellig", "due", "datum", "date", "abgabe");
              return (
                <div className="data-row" key={index}>
                  <span className="grid size-11 place-items-center rounded-lg bg-indigo-50 text-center font-mono text-xs text-indigo-700">SC</span>
                  <div className="data-row-main"><strong>{title || `Aufgabe ${index + 1}`} <span className="pill blue ml-2">SchoolConnect</span></strong><p>{description || "Keine weitere Beschreibung."}{due ? ` · Fällig: ${due}` : ""}</p></div>
                </div>
              );
            }) : <EmptyState title="Alles erledigt" description="Das Schülerportal meldet aktuell keine Hausaufgaben." />}
          </section>
        )}
      </div>
    );
  }

  return <div className="page"><PageHeader eyebrow="Lernplan" title="Hausaufgaben"><select aria-label="Klasse wählen" className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm" value={activeClassId ?? ""} onChange={(event) => selectClass(Number(event.target.value))}>{classes.data?.map((schoolClass) => <option key={schoolClass.id} value={schoolClass.id}>{schoolClass.name}</option>)}</select>{isTeacher && <Button onClick={() => setCreateOpen(true)}>Aufgabe anlegen</Button>}</PageHeader>
    {classes.isLoading || (activeClassId && homework.isLoading) ? <LoadingState /> : classes.isError || homework.isError ? <ErrorState onRetry={() => { classes.refetch(); homework.refetch(); }} /> : !activeClassId ? <EmptyState title="Keine Klasse verfügbar" description="Sobald du einer Klasse zugeordnet bist, findest du hier ihre Aufgaben." /> : <section className="overflow-hidden rounded-xl border border-gray-200 bg-white">{ownHomework.length ? ownHomework.slice().sort((a, b) => a.due_date.localeCompare(b.due_date)).map((item) => {
      const late = new Date(`${item.due_date}T23:59:59`) < new Date();
      const own = ownSubmission.get(item.id);
      const graded = own?.status === "graded";
      return <div className="data-row" key={item.id}>
        <time className={`grid size-11 place-items-center rounded-lg text-center font-mono text-xs ${late ? "bg-red-50 text-red-700" : "bg-indigo-50 text-indigo-700"}`}>{formatDate(item.due_date, { day: "2-digit", month: "short" })}</time>
        <div className="data-row-main"><strong>{item.title} <span className="pill blue ml-2">{subject(item.subject_id)}</span></strong><p>{item.description || "Keine weitere Beschreibung."}</p></div>
        {isTeacher ? <div className="flex gap-3"><button className="text-button" onClick={() => setGrading(item)}>Abgaben</button><button className="text-button" onClick={() => setEditing(item)}>Bearbeiten</button><button className="text-button text-red-600" onClick={() => remove.mutate(item.id)}>Löschen</button></div>
        : graded ? <Button size="sm" variant="secondary" disabled>Benotet ({own!.grade})</Button>
        : own ? <div className="flex items-center gap-2"><label className="text-xs text-gray-500"><input className="hidden" type="file" onChange={(event) => setPendingFiles((prev) => ({ ...prev, [item.id]: event.target.files?.[0] }))} />{pendingFiles[item.id] ? <span className="text-gray-700">{pendingFiles[item.id]!.name}</span> : <span className="cursor-pointer rounded-lg border border-gray-300 px-2.5 py-1.5 hover:bg-gray-50">Neue Datei</span>}</label><Button size="sm" variant="secondary" loading={submit.isPending} onClick={() => submit.mutate({ id: item.id, file: pendingFiles[item.id] })}>Bearbeiten</Button><Button size="sm" variant="ghost" loading={withdraw.isPending} onClick={() => withdraw.mutate({ homeworkId: item.id, submissionId: own.id })}>Zurückziehen</Button></div>
        : <div className="flex items-center gap-2"><label className="text-xs text-gray-500"><input className="hidden" type="file" onChange={(event) => setPendingFiles((prev) => ({ ...prev, [item.id]: event.target.files?.[0] }))} />{pendingFiles[item.id] ? <span className="text-gray-700">{pendingFiles[item.id]!.name}</span> : <span className="cursor-pointer rounded-lg border border-gray-300 px-2.5 py-1.5 hover:bg-gray-50">Datei wählen</span>}</label><Button size="sm" loading={submit.isPending} onClick={() => submit.mutate({ id: item.id, file: pendingFiles[item.id] })}>Abgeben</Button></div>}
      </div>;
    }) : <EmptyState title="Alles erledigt" description={isTeacher ? "Lege die erste Aufgabe für diese Klasse an." : "Für diese Klasse stehen aktuell keine Aufgaben an."} />}</section>}
    {createOpen && <HomeworkDialog subjects={subjects.data ?? []} pending={create.isPending} error={create.isError} onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data as HomeworkInput)} />}{editing && <HomeworkDialog homework={editing} subjects={subjects.data ?? []} pending={update.isPending} error={update.isError} onClose={() => setEditing(undefined)} onSubmit={(data) => update.mutate({ id: editing.id, data: data as UpdateHomeworkInput })} />}{grading && <SubmissionsDialog homework={grading} onClose={() => setGrading(undefined)} />}
  </div>;
}

function field(item: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = item[key];
    if (typeof value === "string" && value.trim()) return value;
    if (typeof value === "number") return String(value);
  }
  return "";
}

function HomeworkDialog({ homework, subjects, onClose, onSubmit, pending, error }: { homework?: Homework; subjects: { id: number; name: string; short: string }[]; onClose: () => void; onSubmit: (data: HomeworkInput | UpdateHomeworkInput) => void; pending: boolean; error: boolean }) {
  const [form, setForm] = useState({ title: homework?.title ?? "", description: homework?.description ?? "", due_date: homework?.due_date ?? new Date().toISOString().slice(0, 10), subject_id: String(homework?.subject_id ?? "") });
  return <div className="modal-backdrop" role="dialog" aria-modal="true"><form className="modal" onSubmit={(event) => { event.preventDefault(); onSubmit({ title: form.title, description: form.description || undefined, due_date: form.due_date, subject_id: Number(form.subject_id) }); }}><h2>{homework ? "Aufgabe bearbeiten" : "Hausaufgabe anlegen"}</h2><div className="form-grid"><label className="full">Titel<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" required value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} /></label><label>Fach<select className="mt-1 w-full rounded-lg border border-gray-300 p-2" required value={form.subject_id} onChange={(event) => setForm({ ...form, subject_id: event.target.value })}><option value="">Auswählen</option>{subjects.map((subject) => <option key={subject.id} value={subject.id}>{subject.name}</option>)}</select></label><label>Fällig am<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" required type="date" value={form.due_date} onChange={(event) => setForm({ ...form, due_date: event.target.value })} /></label><label className="full">Beschreibung<textarea className="mt-1 w-full rounded-lg border border-gray-300 p-2" rows={3} value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label></div>{error && <p className="mt-3 text-sm text-red-600">Die Aufgabe konnte nicht gespeichert werden.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button loading={pending}>{homework ? "Änderungen speichern" : "Aufgabe anlegen"}</Button></div></form></div>;
}

function SubmissionsDialog({ homework, onClose }: { homework: Homework; onClose: () => void }) {
  const client = useQueryClient();
  const submissions = useQuery({ queryKey: ["submissions", homework.id], queryFn: () => schoolApi.submissions(homework.id) });
  const grade = useMutation({ mutationFn: ({ id, value }: { id: number; value: number }) => schoolApi.gradeSubmission(id, value), onSuccess: () => client.invalidateQueries({ queryKey: ["submissions", homework.id] }) });
  return <div className="modal-backdrop" role="dialog" aria-modal="true"><div className="modal max-w-xl"><h2>Abgaben: {homework.title}</h2>{submissions.isLoading ? <LoadingState label="Abgaben werden geladen" /> : submissions.isError ? <ErrorState onRetry={() => submissions.refetch()} /> : !submissions.data?.length ? <p className="py-5 text-sm text-gray-500">Noch keine Abgaben.</p> : <div className="max-h-80 divide-y divide-gray-100 overflow-y-auto">{submissions.data.map((entry) => <GradeRow key={entry.id} submission={entry} pending={grade.isPending} onGrade={(value) => grade.mutate({ id: entry.id, value })} />)}</div>}<div className="modal-actions"><Button variant="ghost" onClick={onClose}>Schließen</Button></div></div></div>;
}

function GradeRow({ submission, pending, onGrade }: { submission: { id: number; student_id: number; status: string; grade?: number; submitted_at?: string; file_id?: number }; pending: boolean; onGrade: (value: number) => void }) {
  const [value, setValue] = useState(String(submission.grade ?? ""));
  const download = () => schoolApi.downloadFile(submission.file_id!).then((response) => {
    const url = URL.createObjectURL(response.data as Blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `abgabe-${submission.student_id}`;
    link.click();
    URL.revokeObjectURL(url);
  });
  return <div className="flex items-center gap-3 py-3"><div className="min-w-0 flex-1"><strong className="text-sm text-gray-900">Schüler:in #{submission.student_id}</strong><p className="text-xs text-gray-500">{submission.submitted_at ? `Abgegeben ${formatDate(submission.submitted_at)}` : "Noch offen"}</p>{submission.file_id && <button className="text-button mt-0.5 text-xs" onClick={download}>Abgabedatei herunterladen</button>}</div><input className="w-16 rounded-lg border border-gray-300 px-2 py-1.5 text-sm" min="1" max="6" step="0.1" type="number" value={value} onChange={(event) => setValue(event.target.value)} /><Button size="sm" variant="secondary" loading={pending} disabled={!value} onClick={() => onGrade(Number(value))}>Benoten</Button></div>;
}
