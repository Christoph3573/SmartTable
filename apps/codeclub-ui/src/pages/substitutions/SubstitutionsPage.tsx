import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type SubstitutionInput } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

const today = new Date().toISOString().slice(0, 10);
const labels = { substitution: "Vertretung", cancellation: "Entfall", room_change: "Raumwechsel", extra: "Zusatzstunde" };

export function SubstitutionsPage() {
  const [date, setDate] = useState(today);
  const [createOpen, setCreateOpen] = useState(false);
  const user = useAuthStore((s) => s.user);
  const queryClient = useQueryClient();
  const substitutions = useQuery({ queryKey: ["substitutions", date], queryFn: () => schoolApi.substitutions({ date_from: date, date_to: date }) });
  const subjects = useQuery({ queryKey: ["subjects"], queryFn: schoolApi.subjects });
  const create = useMutation({ mutationFn: schoolApi.createSubstitution, onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["substitutions"] }); setCreateOpen(false); } });
  const remove = useMutation({ mutationFn: schoolApi.deleteSubstitution, onSuccess: () => queryClient.invalidateQueries({ queryKey: ["substitutions"] }) });
  const canEdit = user?.role === "admin" || user?.role === "teacher";
  const subjectName = (id?: number) => subjects.data?.find((subject) => subject.id === id)?.short ?? "—";

  return <div className="page">
    <PageHeader eyebrow="Tagesplan" title="Vertretungen">
      <input aria-label="Datum wählen" className="rounded-lg border border-[#dfe1da] bg-white px-3 py-2 text-sm" type="date" value={date} onChange={(event) => setDate(event.target.value)} />
      {canEdit && <Button onClick={() => setCreateOpen(true)}>Vertretung eintragen</Button>}
    </PageHeader>
    <section className="surface overflow-hidden">
      <div className="flex items-center justify-between border-b border-[#eeeee8] px-5 py-3"><span className="text-sm font-bold">{formatDate(date, { weekday: "long", day: "2-digit", month: "long" })}</span><span className="text-xs text-[#757a71]">Änderungen erscheinen sofort im Tagesplan.</span></div>
      {substitutions.isLoading ? <LoadingState /> : substitutions.isError ? <ErrorState onRetry={() => substitutions.refetch()} /> : substitutions.data?.length ? substitutions.data.sort((a, b) => a.period - b.period).map((item) => <div className="data-row" key={item.id}>
        <div className="grid size-10 place-items-center rounded-xl bg-[#edf2ef] text-sm font-extrabold text-[#30686b]">{item.period}.</div>
        <div className="data-row-main"><strong>{subjectName(item.subject_id)} · {item.room ? `Raum ${item.room}` : "Raum folgt"}</strong><p>{item.note || (item.type === "cancellation" ? "Diese Stunde entfällt." : "Lehrkraft und Raum wurden aktualisiert.")}</p></div>
        <span className={`pill ${item.type === "cancellation" ? "red" : item.type === "room_change" ? "amber" : "blue"}`}>{labels[item.type]}</span>
        {canEdit && <button className="text-button" onClick={() => remove.mutate(item.id)}>Löschen</button>}
      </div>) : <EmptyState title="Kein geänderter Unterricht" description="Für diesen Tag liegen keine Vertretungen vor." />}
    </section>
    {createOpen && <SubstitutionForm onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data)} pending={create.isPending} error={create.isError} />}
  </div>;
}

function SubstitutionForm({ onClose, onSubmit, pending, error }: { onClose: () => void; onSubmit: (data: SubstitutionInput) => void; pending: boolean; error: boolean }) {
  const [form, setForm] = useState({ date: today, period: "1", type: "substitution" as SubstitutionInput["type"], room: "", note: "" });
  return <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="substitution-title"><form className="modal" onSubmit={(event) => { event.preventDefault(); onSubmit({ ...form, period: Number(form.period), room: form.room || undefined, note: form.note || undefined }); }}><h2 id="substitution-title">Vertretung eintragen</h2><div className="form-grid">
    <label>Datum<input className="mt-1 w-full rounded-lg border p-2" required type="date" value={form.date} onChange={(event) => setForm({ ...form, date: event.target.value })} /></label>
    <label>Stunde<input className="mt-1 w-full rounded-lg border p-2" required min="1" max="12" type="number" value={form.period} onChange={(event) => setForm({ ...form, period: event.target.value })} /></label>
    <label>Art<select className="mt-1 w-full rounded-lg border p-2" value={form.type} onChange={(event) => setForm({ ...form, type: event.target.value as SubstitutionInput["type"] })}><option value="substitution">Vertretung</option><option value="cancellation">Entfall</option><option value="room_change">Raumwechsel</option><option value="extra">Zusatzstunde</option></select></label>
    <label>Raum<input className="mt-1 w-full rounded-lg border p-2" value={form.room} onChange={(event) => setForm({ ...form, room: event.target.value })} /></label>
    <label className="full">Hinweis<input className="mt-1 w-full rounded-lg border p-2" value={form.note} onChange={(event) => setForm({ ...form, note: event.target.value })} /></label>
  </div>{error && <p className="mt-3 text-sm text-red-600">Die Vertretung konnte nicht gespeichert werden.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button type="submit" loading={pending}>Eintragen</Button></div></form></div>;
}
