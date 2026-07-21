import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type CalendarEvent, type EventInput, type UpdateEventInput } from "../../api/school";
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
  const [editing, setEditing] = useState<CalendarEvent>();
  const user = useAuthStore((s) => s.user);
  const client = useQueryClient();
  const range = useMemo(() => ({ start_date: isoDate(month), end_date: isoDate(new Date(month.getFullYear(), month.getMonth() + 1, 0)) }), [month]);
  const events = useQuery({ queryKey: ["events", range], queryFn: () => schoolApi.events(range) });
  const create = useMutation({ mutationFn: schoolApi.createEvent, onSuccess: () => { client.invalidateQueries({ queryKey: ["events"] }); setCreateOpen(false); } });
  const update = useMutation({ mutationFn: ({ id, data }: { id: number; data: UpdateEventInput }) => schoolApi.updateEvent(id, data), onSuccess: () => { client.invalidateQueries({ queryKey: ["events"] }); setEditing(undefined); } });
  const remove = useMutation({ mutationFn: schoolApi.deleteEvent, onSuccess: () => client.invalidateQueries({ queryKey: ["events"] }) });
  const canEdit = user?.role !== "student";
  const days = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const leading = (month.getDay() + 6) % 7;
  const eventDays = new Map<number, CalendarEvent[]>(); (events.data ?? []).forEach((event) => { const day = new Date(event.start_time).getDate(); eventDays.set(day, [...(eventDays.get(day) ?? []), event]); });

  return <div className="page"><PageHeader eyebrow="Schuljahr" title="Kalender"><div className="flex items-center gap-2"><Button variant="ghost" aria-label="Vorheriger Monat" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}>←</Button><span className="min-w-32 text-center text-sm font-bold">{formatDate(isoDate(month), { month: "long", year: "numeric" })}</span><Button variant="ghost" aria-label="Nächster Monat" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}>→</Button></div>{canEdit && <Button onClick={() => setCreateOpen(true)}>Termin anlegen</Button>}</PageHeader>
    {events.isLoading ? <LoadingState /> : events.isError ? <ErrorState onRetry={() => events.refetch()} /> : <div className="grid gap-5 lg:grid-cols-[1.45fr_.75fr]"><section className="surface overflow-hidden p-3"><div className="grid grid-cols-7 text-center text-[.68rem] font-bold uppercase tracking-wider text-gray-400">{["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"].map((day) => <span className="py-2" key={day}>{day}</span>)}</div><div className="grid grid-cols-7">{Array.from({ length: leading + days }, (_, index) => { const day = index - leading + 1; const dayEvents = eventDays.get(day) ?? []; return <div className={`min-h-24 border-t border-gray-100 p-2 ${day < 1 ? "bg-gray-50" : ""}`} key={index}>{day > 0 && <><span className={`grid size-7 place-items-center rounded-full text-sm ${isoDate(new Date()) === isoDate(new Date(month.getFullYear(), month.getMonth(), day)) ? "bg-indigo-600 font-bold text-white" : ""}`}>{day}</span>{dayEvents.slice(0, 2).map((event) => <div className="mt-1 truncate rounded bg-indigo-50 px-1.5 py-1 text-[.65rem] font-semibold leading-tight text-indigo-700" key={event.id} title={event.title}>{event.title}</div>)}{dayEvents.length > 2 && <p className="mt-1 text-[.65rem] text-gray-500">+{dayEvents.length - 2} weitere</p>}</>}</div>; })}</div></section>
      <section className="surface overflow-hidden"><div className="border-b border-gray-100 px-5 py-4"><h2>In diesem Monat</h2></div>{events.data?.length ? events.data.slice().sort((a,b) => a.start_time.localeCompare(b.start_time)).map((event) => <div className="data-row" key={event.id}><time className="w-10 text-center font-mono text-xs text-gray-500">{formatDate(event.start_time, {day:"2-digit",month:"short"})}</time><div className="data-row-main"><strong>{event.title}</strong><p>{event.all_day ? "Ganztägig" : `${new Date(event.start_time).toLocaleTimeString("de-DE", {hour:"2-digit",minute:"2-digit"})} Uhr`} · {typeLabels[event.type]}</p></div>{canEdit && <><button className="text-button" onClick={() => setEditing(event)}>Bearbeiten</button><button className="text-button text-red-600" onClick={() => remove.mutate(event.id)}>Löschen</button></>}</div>) : <EmptyState title="Keine Termine" description="Für diesen Monat sind keine Termine eingetragen." />}</section></div>}
    {createOpen && <EventForm pending={create.isPending} error={create.isError} onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data as EventInput)} />}{editing && <EventForm event={editing} pending={update.isPending} error={update.isError} onClose={() => setEditing(undefined)} onSubmit={(data) => update.mutate({ id: editing.id, data: data as UpdateEventInput })} />}
  </div>;
}

function EventForm({ event, onClose, onSubmit, pending, error }: { event?: CalendarEvent; onClose: () => void; onSubmit: (data: EventInput | UpdateEventInput) => void; pending: boolean; error: boolean }) {
  const start = event ? new Date(event.start_time) : new Date(); const [form, setForm] = useState({ title: event?.title ?? "", date: isoDate(start), time: start.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", hour12: false }), type: (event?.type ?? "event") as EventInput["type"] });
  return <div className="modal-backdrop" role="dialog" aria-modal="true"><form className="modal" onSubmit={(formEvent) => { formEvent.preventDefault(); const startTime = new Date(`${form.date}T${form.time}`); const end = new Date(startTime.getTime() + 60 * 60 * 1000); onSubmit({ title: form.title, type: form.type, start_time: startTime.toISOString(), end_time: end.toISOString() }); }}><h2>{event ? "Termin bearbeiten" : "Termin anlegen"}</h2><div className="form-grid"><label className="full">Titel<input className="mt-1 w-full rounded-lg border p-2" required value={form.title} onChange={(formEvent) => setForm({...form,title:formEvent.target.value})} /></label><label>Datum<input className="mt-1 w-full rounded-lg border p-2" type="date" required value={form.date} onChange={(formEvent) => setForm({...form,date:formEvent.target.value})}/></label><label>Uhrzeit<input className="mt-1 w-full rounded-lg border p-2" type="time" required value={form.time} onChange={(formEvent) => setForm({...form,time:formEvent.target.value})}/></label><label className="full">Kategorie<select className="mt-1 w-full rounded-lg border p-2" value={form.type} onChange={(formEvent) => setForm({...form,type:formEvent.target.value as EventInput["type"]})}>{Object.entries(typeLabels).map(([value,label]) => <option value={value} key={value}>{label}</option>)}</select></label></div>{error && <p className="mt-3 text-sm text-red-600">Der Termin konnte nicht gespeichert werden.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button loading={pending}>Speichern</Button></div></form></div>;
}
