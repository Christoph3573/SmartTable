import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type EventInput } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

const firstOfMonth = () => new Date(new Date().getFullYear(), new Date().getMonth(), 1);
const isoDate = (date: Date) => date.toISOString().slice(0, 10);
const typeLabels = { holiday: "Ferien", exam: "Prüfung", event: "Termin", other: "Sonstiges" };

export function CalendarPage() {
  const [month, setMonth] = useState(firstOfMonth);
  const [createOpen, setCreateOpen] = useState(false);
  const user = useAuthStore((s) => s.user);
  const client = useQueryClient();
  const range = useMemo(() => ({ start_date: isoDate(month), end_date: isoDate(new Date(month.getFullYear(), month.getMonth() + 1, 0)) }), [month]);
  const events = useQuery({ queryKey: ["events", range], queryFn: () => schoolApi.events(range) });
  const create = useMutation({ mutationFn: schoolApi.createEvent, onSuccess: () => { client.invalidateQueries({ queryKey: ["events"] }); setCreateOpen(false); } });
  const remove = useMutation({ mutationFn: schoolApi.deleteEvent, onSuccess: () => client.invalidateQueries({ queryKey: ["events"] }) });
  const canEdit = user?.role !== "student";
  const days = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const leading = (month.getDay() + 6) % 7;
  const eventDays = new Map((events.data ?? []).map((event) => [new Date(event.start_time).getDate(), event]));

  return <div className="page"><PageHeader eyebrow="Schuljahr" title="Kalender"><div className="flex items-center gap-2"><Button variant="ghost" aria-label="Vorheriger Monat" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}>←</Button><span className="min-w-32 text-center text-sm font-bold">{formatDate(isoDate(month), { month: "long", year: "numeric" })}</span><Button variant="ghost" aria-label="Nächster Monat" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}>→</Button></div>{canEdit && <Button onClick={() => setCreateOpen(true)}>Termin anlegen</Button>}</PageHeader>
    {events.isLoading ? <LoadingState /> : events.isError ? <ErrorState onRetry={() => events.refetch()} /> : <div className="grid gap-5 lg:grid-cols-[1.45fr_.75fr]"><section className="surface overflow-hidden p-3"><div className="grid grid-cols-7 text-center text-[.68rem] font-bold uppercase tracking-wider text-[#858b81]">{["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"].map((day) => <span className="py-2" key={day}>{day}</span>)}</div><div className="grid grid-cols-7">{Array.from({ length: leading + days }, (_, index) => { const day = index - leading + 1; const event = eventDays.get(day); return <div className={`min-h-22 border-t border-[#eff0eb] p-2 ${day < 1 ? "bg-[#fafaf7]" : ""}`} key={index}>{day > 0 && <><span className={`grid size-7 place-items-center rounded-full text-sm ${isoDate(new Date()) === isoDate(new Date(month.getFullYear(), month.getMonth(), day)) ? "bg-[#336e72] font-bold text-white" : ""}`}>{day}</span>{event && <div className="mt-1 rounded-md bg-[#e8f0ed] px-1.5 py-1 text-[.65rem] font-bold leading-tight text-[#2c696a]" title={event.title}>{event.title}</div>}</>}</div>; })}</div></section>
      <section className="surface overflow-hidden"><div className="border-b border-[#eeeee8] px-5 py-4"><h2>In diesem Monat</h2></div>{events.data?.length ? events.data.sort((a,b) => a.start_time.localeCompare(b.start_time)).map((event) => <div className="data-row" key={event.id}><time className="w-10 text-center font-mono text-xs text-[#6a7068]">{formatDate(event.start_time, {day:"2-digit",month:"short"})}</time><div className="data-row-main"><strong>{event.title}</strong><p>{event.all_day ? "Ganztägig" : `${new Date(event.start_time).toLocaleTimeString("de-DE", {hour:"2-digit",minute:"2-digit"})} Uhr`} · {typeLabels[event.type]}</p></div>{canEdit && <button className="text-button" onClick={() => remove.mutate(event.id)}>Löschen</button>}</div>) : <EmptyState title="Keine Termine" description="Für diesen Monat sind keine Termine eingetragen." />}</section></div>}
    {createOpen && <EventForm pending={create.isPending} error={create.isError} onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data)} />}
  </div>;
}

function EventForm({ onClose, onSubmit, pending, error }: { onClose: () => void; onSubmit: (data: EventInput) => void; pending: boolean; error: boolean }) {
  const date = isoDate(new Date()); const [form, setForm] = useState({ title: "", date, time: "08:00", type: "event" as EventInput["type"] });
  return <div className="modal-backdrop" role="dialog" aria-modal="true"><form className="modal" onSubmit={(event) => { event.preventDefault(); const start = new Date(`${form.date}T${form.time}`); const end = new Date(start.getTime() + 60 * 60 * 1000); onSubmit({ title: form.title, type: form.type, start_time: start.toISOString(), end_time: end.toISOString() }); }}><h2>Termin anlegen</h2><div className="form-grid"><label className="full">Titel<input className="mt-1 w-full rounded-lg border p-2" required value={form.title} onChange={(event) => setForm({...form,title:event.target.value})} /></label><label>Datum<input className="mt-1 w-full rounded-lg border p-2" type="date" required value={form.date} onChange={(event) => setForm({...form,date:event.target.value})}/></label><label>Uhrzeit<input className="mt-1 w-full rounded-lg border p-2" type="time" required value={form.time} onChange={(event) => setForm({...form,time:event.target.value})}/></label><label className="full">Kategorie<select className="mt-1 w-full rounded-lg border p-2" value={form.type} onChange={(event) => setForm({...form,type:event.target.value as EventInput["type"]})}>{Object.entries(typeLabels).map(([value,label]) => <option value={value} key={value}>{label}</option>)}</select></label></div>{error && <p className="mt-3 text-sm text-red-600">Der Termin konnte nicht gespeichert werden.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button loading={pending}>Speichern</Button></div></form></div>;
}
