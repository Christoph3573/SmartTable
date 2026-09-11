import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type ChatContact, type CreateChannelInput } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { useChatUiStore } from "../../store/chatUiStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

const channelLabel = (channel: { name?: string; type: string }) => channel.name || ({ direct: "Direktnachricht", class: "Klassenchat", group: "Gruppe" }[channel.type] ?? "Chat");
const channelType = (type: string) => ({ direct: "Direkt", class: "Klasse", group: "Gruppe" }[type] ?? "Chat");

export function ChatPage() {
  const [channelId, setChannelId] = useState<number>();
  const [text, setText] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const client = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const channels = useQuery({ queryKey: ["channels"], queryFn: schoolApi.channels });
  const contacts = useQuery({ queryKey: ["chat-contacts"], queryFn: schoolApi.chatContacts });
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const activeChannelId = channelId ?? channels.data?.[0]?.id;
  const setActiveChannelId = useChatUiStore((state) => state.setActiveChannelId);
  useEffect(() => {
    setActiveChannelId(activeChannelId);
    return () => setActiveChannelId(undefined);
  }, [activeChannelId, setActiveChannelId]);
  const messages = useQuery({ queryKey: ["messages", activeChannelId], queryFn: () => schoolApi.messages(activeChannelId!), enabled: Boolean(activeChannelId) });
  useEffect(() => {
    // Fetching messages marks the channel read as a side effect on the backend -
    // once that lands, refresh the unread badges to reflect it.
    if (messages.data) client.invalidateQueries({ queryKey: ["channels"] });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeChannelId, messages.dataUpdatedAt]);
  const send = useMutation({ mutationFn: () => schoolApi.sendMessage(activeChannelId!, text.trim()), onSuccess: () => { setText(""); client.invalidateQueries({ queryKey: ["messages", activeChannelId] }); } });
  const create = useMutation({ mutationFn: schoolApi.createChannel, onSuccess: (channel) => { client.invalidateQueries({ queryKey: ["channels"] }); setChannelId(channel.id); setCreateOpen(false); } });
  const selected = channels.data?.find((channel) => channel.id === activeChannelId);
  const contactNames = useMemo(() => new Map((contacts.data ?? []).map((contact) => [contact.id, `${contact.first_name} ${contact.last_name}`])), [contacts.data]);

  return <div className="page">
    <PageHeader eyebrow="Austausch" title="Nachrichten"><Button onClick={() => setCreateOpen(true)}>Neue Unterhaltung</Button></PageHeader>
    {channels.isLoading ? <LoadingState /> : channels.isError ? <ErrorState onRetry={() => channels.refetch()} /> : !channels.data?.length ? <EmptyState title="Starte eine Unterhaltung" description="Schreibe einer Person, eröffne eine Gruppe oder lege einen Klassenchat an."><Button className="mt-3" onClick={() => setCreateOpen(true)}>Unterhaltung starten</Button></EmptyState> : <div className="grid min-h-[580px] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm md:grid-cols-[17rem_minmax(0,1fr)]">
      <aside className="border-b border-gray-200 bg-gray-50/70 md:border-b-0 md:border-r"><div className="flex items-center justify-between border-b border-gray-200 px-4 py-4"><h2 className="text-sm font-semibold text-gray-900">Unterhaltungen</h2><button className="text-sm font-semibold text-indigo-700 hover:text-indigo-900" onClick={() => setCreateOpen(true)}>Neu</button></div><nav className="p-2">{channels.data.map((channel) => <button key={channel.id} onClick={() => setChannelId(channel.id)} className={`mb-1 w-full rounded-lg border px-3 py-3 text-left transition-colors ${channel.id === activeChannelId ? "border-indigo-200 bg-white text-indigo-950 shadow-sm" : "border-transparent text-gray-700 hover:bg-white"}`}><span className="block truncate text-sm font-semibold">{channelLabel(channel)}</span><span className="mt-1 block text-xs text-gray-500">{channelType(channel.type)}</span></button>)}</nav></aside>
      <section className="flex min-h-[460px] flex-col"><header className="flex items-center justify-between border-b border-gray-200 px-5 py-4"><div><h2 className="text-base font-semibold text-gray-900">{selected ? channelLabel(selected) : "Unterhaltung"}</h2><p className="mt-0.5 text-xs text-gray-500">Neue Nachrichten erscheinen automatisch.</p></div><span className="rounded-full bg-indigo-50 px-2.5 py-1 text-xs font-medium text-indigo-700">{selected && channelType(selected.type)}</span></header>{messages.isLoading ? <LoadingState label="Nachrichten werden geladen" /> : messages.isError ? <ErrorState onRetry={() => messages.refetch()} /> : <><div className="flex flex-1 flex-col justify-end gap-4 overflow-y-auto bg-[#fafafa] p-5">{messages.data?.length ? messages.data.slice().reverse().map((message) => { const own = message.sender_id === user?.id; const sender = own ? "Du" : contactNames.get(message.sender_id) ?? "Teilnehmer:in"; return <div className={`max-w-[80%] ${own ? "self-end" : "self-start"}`} key={message.id}><p className={`mb-1 px-1 text-[11px] font-medium ${own ? "text-right text-indigo-700" : "text-gray-500"}`}>{sender}</p><div className={`rounded-2xl px-4 py-3 text-sm leading-6 ${own ? "rounded-br-sm bg-indigo-600 text-white" : "rounded-bl-sm bg-white text-gray-800 shadow-sm ring-1 ring-gray-200"}`}><p className="whitespace-pre-wrap break-words">{message.content}</p><time className={`mt-1.5 block text-[.65rem] ${own ? "text-indigo-100" : "text-gray-400"}`}>{formatDate(message.created_at, { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</time></div></div>; }) : <EmptyState title="Schreib die erste Nachricht" description="Diese Unterhaltung hat noch keine Nachrichten." />}</div><form className="flex gap-2 border-t border-gray-200 bg-white p-3" onSubmit={(event) => { event.preventDefault(); if (text.trim()) send.mutate(); }}><input aria-label="Nachricht" className="min-w-0 flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100" placeholder="Nachricht schreiben" value={text} onChange={(event) => setText(event.target.value)} /><Button type="submit" loading={send.isPending} disabled={!text.trim()}>Senden</Button></form>{send.isError && <p className="px-4 pb-3 text-xs text-red-600">Die Nachricht konnte nicht gesendet werden.</p>}</>}</section>
    </div>}
    {createOpen && <CreateChannelDialog contacts={contacts.data ?? []} classes={classes.data ?? []} canCreateClass={user?.role === "teacher" || user?.role === "admin"} pending={create.isPending} error={create.isError} onClose={() => setCreateOpen(false)} onSubmit={(data) => create.mutate(data)} />}
  </div>;
}

function CreateChannelDialog({ contacts, classes, canCreateClass, pending, error, onClose, onSubmit }: { contacts: ChatContact[]; classes: { id: number; name: string }[]; canCreateClass: boolean; pending: boolean; error: boolean; onClose: () => void; onSubmit: (data: CreateChannelInput) => void }) {
  const [kind, setKind] = useState<"direct" | "group" | "class">("direct");
  const [contactId, setContactId] = useState("");
  const [memberIds, setMemberIds] = useState<number[]>([]);
  const [classId, setClassId] = useState("");
  const [name, setName] = useState("");
  const submit = () => {
    if (kind === "direct") { const contact = contacts.find((entry) => entry.id === Number(contactId)); if (contact) onSubmit({ type: "direct", name: `${contact.first_name} ${contact.last_name}`, member_ids: [contact.id] }); return; }
    if (kind === "group") { onSubmit({ type: "group", name, member_ids: memberIds }); return; }
    const schoolClass = classes.find((entry) => entry.id === Number(classId)); if (schoolClass) onSubmit({ type: "class", name: `Klasse ${schoolClass.name}`, class_id: schoolClass.id });
  };
  return <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="new-chat-title"><form className="modal max-w-xl" onSubmit={(event) => { event.preventDefault(); submit(); }}><h2 id="new-chat-title">Neue Unterhaltung</h2><p className="mb-4 text-sm text-gray-500">Du erreichst Personen aus deinen gemeinsamen Klassen.</p><div className="mb-5 flex gap-2 border-b border-gray-200"><Choice active={kind === "direct"} onClick={() => setKind("direct")}>Direkt</Choice><Choice active={kind === "group"} onClick={() => setKind("group")}>Gruppe</Choice>{canCreateClass && <Choice active={kind === "class"} onClick={() => setKind("class")}>Klasse</Choice>}</div>{kind === "direct" && <label className="block text-sm font-medium text-gray-700">Person<select className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required value={contactId} onChange={(event) => setContactId(event.target.value)}><option value="">Person auswählen</option>{contacts.map((contact) => <option key={contact.id} value={contact.id}>{contact.last_name}, {contact.first_name} · {contact.role === "teacher" ? "Lehrkraft" : "Schüler:in"}</option>)}</select></label>}{kind === "group" && <><label className="block text-sm font-medium text-gray-700">Gruppenname<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={name} onChange={(event) => setName(event.target.value)} placeholder="z. B. Projekt Biologie" /></label><fieldset className="mt-4"><legend className="text-sm font-medium text-gray-700">Mitglieder</legend><div className="mt-2 max-h-44 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2">{contacts.map((contact) => <label className="flex cursor-pointer items-center gap-3 rounded px-2 py-2 text-sm hover:bg-gray-50" key={contact.id}><input type="checkbox" checked={memberIds.includes(contact.id)} onChange={(event) => setMemberIds(event.target.checked ? [...memberIds, contact.id] : memberIds.filter((id) => id !== contact.id))} /><span>{contact.first_name} {contact.last_name}</span><span className="ml-auto text-xs text-gray-400">{contact.role}</span></label>)}{!contacts.length && <p className="p-2 text-sm text-gray-500">Noch keine erreichbaren Personen.</p>}</div></fieldset></>}{kind === "class" && <label className="block text-sm font-medium text-gray-700">Klasse<select className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required value={classId} onChange={(event) => setClassId(event.target.value)}><option value="">Klasse auswählen</option>{classes.map((schoolClass) => <option key={schoolClass.id} value={schoolClass.id}>{schoolClass.name}</option>)}</select></label>}{error && <p className="mt-3 text-sm text-red-600">Die Unterhaltung konnte nicht angelegt werden. Prüfe Auswahl und Berechtigung.</p>}<div className="modal-actions"><Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button><Button disabled={kind === "direct" ? !contactId : kind === "group" ? !name.trim() || !memberIds.length : !classId} loading={pending}>Unterhaltung anlegen</Button></div></form></div>;
}

function Choice({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return <button className={`border-b-2 px-2 pb-2 text-sm font-medium ${active ? "border-indigo-600 text-indigo-700" : "border-transparent text-gray-500 hover:text-gray-800"}`} type="button" onClick={onClick}>{children}</button>;
}
