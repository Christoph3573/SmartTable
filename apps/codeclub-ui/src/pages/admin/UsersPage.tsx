import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type CreateUserInput, type SchoolUser } from "../../api/school";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui/Page";

const roles = ["student", "teacher", "admin"] as const;
const roleLabel: Record<(typeof roles)[number], string> = { student: "Schüler:in", teacher: "Lehrkraft", admin: "Admin" };

export function UsersPage() {
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const [mode, setMode] = useState<"single" | "bulk" | null>(null);
  const [editing, setEditing] = useState<SchoolUser>();
  const users = useQuery({ queryKey: ["users"], queryFn: schoolApi.users });
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const refresh = () => { client.invalidateQueries({ queryKey: ["users"] }); client.invalidateQueries({ queryKey: ["classes"] }); };
  const remove = useMutation({ mutationFn: schoolApi.deleteUser, onSuccess: refresh });
  const create = useMutation({ mutationFn: schoolApi.createUser, onSuccess: () => { refresh(); setMode(null); } });
  const update = useMutation({ mutationFn: ({ id, data }: { id: number; data: Parameters<typeof schoolApi.updateUser>[1] }) => schoolApi.updateUser(id, data), onSuccess: () => { refresh(); setEditing(undefined); } });
  const bulk = useMutation({
    mutationFn: async ({ users: entries, classId }: { users: CreateUserInput[]; classId?: number }) => {
      const created = await schoolApi.createUsersBulk(entries);
      if (classId) {
        await Promise.all(created.map((user) => user.role === "student" ? schoolApi.addClassMember(classId, user.id) : user.role === "teacher" ? schoolApi.addClassTeacher(classId, user.id) : Promise.resolve()));
      }
      return created;
    },
    onSuccess: () => { refresh(); setMode(null); },
  });
  const filtered = useMemo(() => users.data?.filter((user) => `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase().includes(search.toLowerCase())) ?? [], [users.data, search]);

  return <div className="p-8">
    <div className="mb-8 flex flex-wrap items-center justify-between gap-4"><div><h1 className="text-2xl font-semibold text-gray-900">Benutzerverwaltung</h1><p className="mt-2 text-sm text-gray-500">Konten anlegen, Rollen verwalten und mehrere Nutzer:innen auf einmal importieren.</p></div><div className="flex gap-2"><Button variant="secondary" onClick={() => setMode("bulk")}>Massenanlage</Button><Button onClick={() => setMode("single")}>Benutzer anlegen</Button></div></div>
    {mode === "single" && <UserCreateForm pending={create.isPending} error={create.isError} onCancel={() => setMode(null)} onSubmit={(data) => create.mutate(data)} />}
    {mode === "bulk" && <BulkCreateForm classes={classes.data ?? []} pending={bulk.isPending} error={bulk.isError} onCancel={() => setMode(null)} onSubmit={(data) => bulk.mutate(data)} />}
    {editing && <UserEditForm user={editing} pending={update.isPending} error={update.isError} onCancel={() => setEditing(undefined)} onSubmit={(data) => update.mutate({ id: editing.id, data })} />}

    <section className="overflow-hidden rounded-xl border border-gray-200 bg-white">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4"><strong className="text-sm text-gray-900">{users.data?.length ?? 0} aktive Benutzer</strong><input className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm sm:w-64" placeholder="Suchen nach Name oder E-Mail" value={search} onChange={(event) => setSearch(event.target.value)} /></div>
      {users.isLoading ? <div className="p-6"><LoadingState /></div> : users.isError ? <div className="p-6"><ErrorState onRetry={() => users.refetch()} /></div> : !filtered.length ? <EmptyState title="Keine passenden Benutzer" description="Passe die Suche an oder lege ein neues Konto an." /> : <div className="overflow-x-auto"><table className="min-w-full text-left text-sm"><thead className="bg-gray-50 text-xs uppercase tracking-wide text-gray-500"><tr><th className="px-5 py-3 font-semibold">Name</th><th className="px-5 py-3 font-semibold">E-Mail</th><th className="px-5 py-3 font-semibold">Rolle</th><th className="px-5 py-3" /></tr></thead><tbody>{filtered.map((user) => <tr className="border-t border-gray-100" key={user.id}><td className="px-5 py-3 font-medium text-gray-900">{user.first_name} {user.last_name}</td><td className="px-5 py-3 text-gray-600">{user.email}</td><td className="px-5 py-3"><span className="rounded-full bg-indigo-50 px-2.5 py-1 text-xs font-medium text-indigo-700">{roleLabel[user.role as keyof typeof roleLabel]}</span></td><td className="whitespace-nowrap px-5 py-3 text-right"><button className="mr-3 text-sm font-medium text-indigo-700 hover:underline" onClick={() => setEditing(user)}>Bearbeiten</button><button className="text-sm font-medium text-red-600 hover:underline" onClick={() => remove.mutate(user.id)} disabled={remove.isPending}>Deaktivieren</button></td></tr>)}</tbody></table></div>}
    </section>
  </div>;
}

function UserFields({ value, onChange, includePassword = true }: { value: Partial<CreateUserInput>; onChange: (value: Partial<CreateUserInput>) => void; includePassword?: boolean }) {
  return <div className="grid gap-4 md:grid-cols-2"><label className="text-sm font-medium text-gray-700">Vorname<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={value.first_name ?? ""} onChange={(event) => onChange({ ...value, first_name: event.target.value })} /></label><label className="text-sm font-medium text-gray-700">Nachname<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={value.last_name ?? ""} onChange={(event) => onChange({ ...value, last_name: event.target.value })} /></label><label className="text-sm font-medium text-gray-700 md:col-span-2">E-Mail<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" type="email" required value={value.email ?? ""} onChange={(event) => onChange({ ...value, email: event.target.value })} /></label><label className="text-sm font-medium text-gray-700">Rolle<select className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" value={value.role ?? "student"} onChange={(event) => onChange({ ...value, role: event.target.value as CreateUserInput["role"] })}>{roles.map((role) => <option key={role} value={role}>{roleLabel[role]}</option>)}</select></label>{includePassword && <label className="text-sm font-medium text-gray-700">Startpasswort<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" type="password" minLength={8} required value={value.password ?? ""} onChange={(event) => onChange({ ...value, password: event.target.value })} /></label>}</div>;
}

function UserCreateForm({ onSubmit, onCancel, pending, error }: { onSubmit: (value: CreateUserInput) => void; onCancel: () => void; pending: boolean; error: boolean }) {
  const [value, setValue] = useState<Partial<CreateUserInput>>({ role: "student" });
  return <form className="mb-6 rounded-xl border border-indigo-200 bg-indigo-50 p-5" onSubmit={(event) => { event.preventDefault(); onSubmit(value as CreateUserInput); }}><h2 className="mb-4 text-base font-semibold text-gray-900">Benutzer anlegen</h2><UserFields value={value} onChange={setValue} /><FormActions pending={pending} error={error} onCancel={onCancel} submit="Benutzer anlegen" /></form>;
}

function UserEditForm({ user, onSubmit, onCancel, pending, error }: { user: SchoolUser; onSubmit: (value: { first_name: string; last_name: string; role: CreateUserInput["role"]; password?: string }) => void; onCancel: () => void; pending: boolean; error: boolean }) {
  const [value, setValue] = useState({ first_name: user.first_name, last_name: user.last_name, role: user.role as CreateUserInput["role"], password: "" });
  return <form className="mb-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm" onSubmit={(event) => { event.preventDefault(); onSubmit({ ...value, password: value.password || undefined }); }}><h2 className="mb-1 text-base font-semibold text-gray-900">{user.first_name} {user.last_name} bearbeiten</h2><p className="mb-4 text-sm text-gray-500">{user.email}</p><div className="grid gap-4 md:grid-cols-2"><label className="text-sm font-medium text-gray-700">Vorname<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={value.first_name} onChange={(event) => setValue({ ...value, first_name: event.target.value })} /></label><label className="text-sm font-medium text-gray-700">Nachname<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={value.last_name} onChange={(event) => setValue({ ...value, last_name: event.target.value })} /></label><label className="text-sm font-medium text-gray-700">Rolle<select className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" value={value.role} onChange={(event) => setValue({ ...value, role: event.target.value as CreateUserInput["role"] })}>{roles.map((role) => <option key={role} value={role}>{roleLabel[role]}</option>)}</select></label><label className="text-sm font-medium text-gray-700">Neues Passwort <span className="font-normal text-gray-400">(optional)</span><input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" type="password" minLength={8} value={value.password} onChange={(event) => setValue({ ...value, password: event.target.value })} /></label></div><FormActions pending={pending} error={error} onCancel={onCancel} submit="Änderungen speichern" /></form>;
}

function BulkCreateForm({ classes, onSubmit, onCancel, pending, error }: { classes: { id: number; name: string }[]; onSubmit: (value: { users: CreateUserInput[]; classId?: number }) => void; onCancel: () => void; pending: boolean; error: boolean }) {
  const [rows, setRows] = useState(""); const [classId, setClassId] = useState(""); const [parseError, setParseError] = useState("");
  const submit = () => { const parsed = rows.split(/\r?\n/).map((line) => line.trim()).filter(Boolean).map((line) => line.split(/[;,]/).map((cell) => cell.trim())); if (!parsed.length) { setParseError("Füge mindestens eine Zeile ein."); return; } const invalid = parsed.find((columns) => columns.length !== 5 || !roles.includes(columns[3] as (typeof roles)[number])); if (invalid) { setParseError("Jede Zeile braucht fünf Werte und eine gültige Rolle (student, teacher oder admin)."); return; } setParseError(""); onSubmit({ users: parsed.map(([email, first_name, last_name, role, password]) => ({ email, first_name, last_name, role: role as CreateUserInput["role"], password })), classId: classId ? Number(classId) : undefined }); };
  return <form className="mb-6 rounded-xl border border-indigo-200 bg-indigo-50 p-5" onSubmit={(event) => { event.preventDefault(); submit(); }}><h2 className="text-base font-semibold text-gray-900">Mehrere Benutzer anlegen</h2><p className="mt-1 text-sm text-gray-600">Eine Zeile pro Benutzer: <code className="rounded bg-white px-1">E-Mail; Vorname; Nachname; Rolle; Startpasswort</code></p><textarea className="mt-4 min-h-44 w-full rounded-lg border border-gray-300 bg-white p-3 font-mono text-sm" value={rows} onChange={(event) => setRows(event.target.value)} placeholder={"maria.muster@schule.de; Maria; Muster; student; Startpasswort1\njonas.lehrer@schule.de; Jonas; Lehrer; teacher; Startpasswort2"} /><label className="mt-4 block text-sm font-medium text-gray-700">Direkt einer Klasse zuordnen <span className="font-normal text-gray-400">(optional)</span><select className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" value={classId} onChange={(event) => setClassId(event.target.value)}><option value="">Keine direkte Zuordnung</option>{classes.map((schoolClass) => <option key={schoolClass.id} value={schoolClass.id}>{schoolClass.name}</option>)}</select></label><p className="mt-2 text-xs text-gray-500">Schüler:innen werden als Mitglieder, Lehrkräfte als Lehrkräfte der gewählten Klasse zugeordnet.</p>{parseError && <p className="mt-3 text-sm text-red-600">{parseError}</p>}<FormActions pending={pending} error={error} onCancel={onCancel} submit="Benutzer anlegen" /></form>;
}

function FormActions({ pending, error, onCancel, submit }: { pending: boolean; error: boolean; onCancel: () => void; submit: string }) { return <div className="mt-5 flex items-center gap-3"><Button type="button" variant="ghost" onClick={onCancel}>Abbrechen</Button><Button loading={pending}>{submit}</Button>{error && <span className="text-sm text-red-600">Die Änderungen konnten nicht gespeichert werden.</span>}</div>; }
