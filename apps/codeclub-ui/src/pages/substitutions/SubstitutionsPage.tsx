import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import axios from "axios";
import { schoolApi, type Substitution, type SubstitutionInput, type UpdateSubstitutionInput } from "../../api/school";
import { schoolConnectApi, scSubstitutionLabel, scSubstitutionCourse } from "../../api/schoolconnect";
import { useAuthStore } from "../../store/authStore";
import { useSettingsStore } from "../../store/settingsStore";
import { useCourseVisibility } from "../../lib/useCourseVisibility";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

const today = new Date().toISOString().slice(0, 10);
const labels = { substitution: "Vertretung", cancellation: "Entfall", room_change: "Raumwechsel", extra: "Zusatzstunde" };

function isAuthError(err: unknown): boolean {
  return axios.isAxiosError(err) && err.response?.status === 401;
}

export function SubstitutionsPage() {
  const [date, setDate] = useState(today);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Substitution>();
  const user = useAuthStore((s) => s.user);
  const provider = useSettingsStore((s) => s.provider);
  const queryClient = useQueryClient();
  const isExternal = provider === "schoolconnect";
  const visibility = useCourseVisibility();
  const substitutions = useQuery({
    queryKey: ["substitutions", date],
    queryFn: () => schoolApi.substitutions({ date_from: date, date_to: date }),
    enabled: !isExternal,
  });
  const external = useQuery({
    queryKey: ["schoolconnect", "vertretungsplan", date],
    queryFn: () => schoolConnectApi.vertretungsplan({ datum: date }),
    enabled: isExternal,
    retry: 1,
  });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects, enabled: !isExternal });
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes, enabled: !isExternal });
  const create = useMutation({ mutationFn: schoolApi.createSubstitution, onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["substitutions"] }); setCreateOpen(false); } });
  const update = useMutation({ mutationFn: ({ id, data }: { id: number; data: UpdateSubstitutionInput }) => schoolApi.updateSubstitution(id, data), onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["substitutions"] }); setEditing(undefined); } });
  const remove = useMutation({ mutationFn: schoolApi.deleteSubstitution, onSuccess: () => queryClient.invalidateQueries({ queryKey: ["substitutions"] }) });
  const canEdit = !isExternal && (user?.role === "teacher" || user?.role === "school_admin" || user?.role === "superadmin" || user?.role === "admin");
  const subjectName = (id?: number) => subjects.data?.find((subject) => subject.id === id)?.short ?? "—";
  const externalEntries = (external.data?.eintraege ?? []).filter(
    (item) => !visibility.hiddenKurs(scSubstitutionCourse(item))
  );
  const ownSubstitutions = (substitutions.data ?? []).filter(
    (item) => !(item.subject_id != null && visibility.hiddenSubject(item.subject_id))
  );

  if (isExternal) {
    return (
      <div className="page">
        <PageHeader eyebrow="Tagesplan · SchoolConnect" title="Vertretungen">
          <input aria-label="Datum wählen" className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm" type="date" value={date} onChange={(event) => setDate(event.target.value)} />
        </PageHeader>
        <section className="overflow-hidden rounded-xl border border-gray-200 bg-white">
          <div className="flex items-center justify-between border-b border-gray-100 px-5 py-3">
            <span className="text-sm font-bold">{formatDate(date, { weekday: "long", day: "2-digit", month: "long" })}</span>
            <span className="text-xs text-gray-500">Quelle: Schülerportal via SchoolConnect — schreibgeschützt.</span>
          </div>
          {external.isLoading ? <LoadingState label="Vertretungsplan wird geladen" /> : external.isError ? (
            isAuthError(external.error) ? (
              <div className="px-5 py-9 text-center text-sm text-gray-500">
                <strong className="block text-gray-900 dark:text-white">Nicht beim Schülerportal angemeldet</strong>
                <p className="mt-1">Bitte in den <Link className="font-semibold text-indigo-700 hover:underline" to="/settings">Einstellungen → SchoolConnect</Link> anmelden.</p>
                <div className="mt-3"><Button size="sm" variant="secondary" onClick={() => external.refetch()}>Erneut versuchen</Button></div>
              </div>
            ) : (
              <ErrorState onRetry={() => external.refetch()} message="Der Vertretungsplan konnte von SchoolConnect nicht geladen werden." />
            )
          ) : externalEntries.length ? (
            externalEntries.map((item, index) => {
              const label = scSubstitutionLabel(item);
              return (
                <div className="data-row" key={index}>
                  <div className="grid size-10 place-items-center rounded-lg bg-indigo-50 text-sm font-extrabold text-indigo-700">{label.period}</div>
                  <div className="data-row-main"><strong>{label.title}</strong><p>{label.detail}</p></div>
                  <span className="pill blue">SchoolConnect</span>
                </div>
              );
            })
          ) : (
            <EmptyState title="Kein geänderter Unterricht" description="Für diesen Tag meldet das Schülerportal keine Vertretungen (bzw. alle abgewählten Kurse sind ausgeblendet)." />
          )}
        </section>
      </div>
    );
  }

  return <div className="page"><PageHeader eyebrow="Tagesplan" title="Vertretungen"><input aria-label="Datum wählen" className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm" type="date" value={date} onChange={(event) => setDate(event.target.value)} />{canEdit && <Button onClick={() => setCreateOpen(true)}>Vertretung eintragen</Button>}</PageHeader><section className="overflow-hidden rounded-xl border border-gray-200 bg-white"><div className="flex items-center justify-between border-b border-gray-100 px-5 py-3"><span className="text-sm font-bold">{formatDate(date, { weekday: "long", day: "2-digit", month: "long" })}</span><span className="text-xs text-gray-500">Änderungen erscheinen sofort im Tagesplan.</span></div>{substitutions.isLoading ? <LoadingState /> : substitutions.isError ? <ErrorState onRetry={() => substitutions.refetch()} /> : ownSubstitutions.length ? ownSubstitutions.slice().sort((a, b) => a.period - b.period).map((item) => <div className="data-row" key={item.id}><div className="grid size-10 place-items-center rounded-lg bg-indigo-50 text-sm font-extrabold text-indigo-700">{item.period}.</div><div className="data-row-main"><strong>{subjectName(item.subject_id)} · {item.room ? `Raum ${item.room}` : "Raum folgt"}</strong><p>{item.note || (item.type === "cancellation" ? "Diese Stunde entfällt." : "Lehrkraft und Raum wurden aktualisiert.")}</p></div><span className={`pill ${item.type === "cancellation" ? "red" : item.type === "room_change" ? "amber" : "blue"}`}>{labels[item.type]}</span>{canEdit && <><button className="text-button" onClick={() => setEditing(item)}>Bearbeiten</button><button className="text-button text-red-600" onClick={() => remove.mutate(item.id)}>Löschen</button></>}</div>) : <EmptyState title="Kein geänderter Unterricht" description="Für diesen Tag liegen keine Vertretungen vor." />}</section>{createOpen && <SubstitutionDialog classes={classes.data ?? []} subjects={subjects.data ?? []} onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data as SubstitutionInput)} pending={create.isPending} error={create.isError} />}{editing && <SubstitutionDialog item={editing} classes={classes.data ?? []} subjects={subjects.data ?? []} onClose={() => setEditing(undefined)} onSubmit={(data) => update.mutate({ id: editing.id, data: data as UpdateSubstitutionInput })} pending={update.isPending} error={update.isError} />}
  </div>;
}

function SubstitutionDialog({ item, classes, subjects, onClose, onSubmit, pending, error }: { item?: Substitution; classes: { id: number; name: string }[]; subjects: { id: number; name: string }[]; onClose: () => void; onSubmit: (data: SubstitutionInput | UpdateSubstitutionInput) => void; pending: boolean; error: boolean }) {
  const [form, setForm] = useState({ date: item?.date ?? today, period: String(item?.period ?? 1), type: (item?.type ?? "substitution") as SubstitutionInput["type"], class_id: String(item?.class_id ?? ""), subject_id: String(item?.subject_id ?? ""), room: item?.room ?? "", note: item?.note ?? "" });
  return <div className="modal-backdrop" role="dialog" aria-modal="true"><form className="modal" onSubmit={(event) => { event.preventDefault(); onSubmit({ date: form.date, period: Number(form.period), type: form.type, class_id: form.class_id ? Number(form.class_id) : undefined, subject_id: form.subject_id ? Number(form.subject_id) : undefined, room: form.room || undefined, note: form.note || undefined }); }}><h2>{item ? "Vertretung bearbeiten" : "Vertretung eintragen"}</h2><div className="form-grid"><label>Datum<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" required type="date" value={form.date} onChange={(event) => setForm({ ...form, date: event.target.value })} /></label><label>Stunde<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" required min="1" max="12" type="number" value={form.period} onChange={(event) => setForm({ ...form, period: event.target.value })} /></label><label>Klasse<select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.class_id} onChange={(event) => setForm({ ...form, class_id: event.target.value })}><option value="">Schulweit</option>{classes.map((schoolClass) => <option key={schoolClass.id} value={schoolClass.id}>{schoolClass.name}</option>)}</select></label><label>Fach<select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.subject_id} onChange={(event) => setForm({ ...form, subject_id: event.target.value })}><option value="">Nicht angegeben</option>{subjects.map((subject) => <option key={subject.id} value={subject.id}>{subject.name}</option>)}</select></label><label>Art<select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.type} onChange={(event) => setForm({ ...form, type: event.target.value as SubstitutionInput["type"] })}><option value="substitution">Vertretung</option><option value="cancellation">Entfall</option><option value="room_change">Raumwechsel</option><option value="extra">Zusatzstunde</option></select></label><label>Raum<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.room} onChange={(event) => setForm({ ...form, room: event.target.value })} /></label><label className="full">Notiz<input className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.note} onChange={(event) => setForm({ ...form, note: event.target.value })} /></label></div>{error && <p className="mt-3 text-sm text-red-600">Die Vertretung konnte nicht gespeichert werden.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button type="submit" loading={pending}>{item ? "Änderungen speichern" : "Eintragen"}</Button></div></form></div>;
}
