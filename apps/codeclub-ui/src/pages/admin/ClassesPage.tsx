import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi, type SchoolClass } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui/Page";

const currentSchoolYear = "2026/27";

export function ClassesPage() {
  const client = useQueryClient();
  const role = useAuthStore((state) => state.user?.role);
  const canCreate = role === "superadmin" || role === "admin" || role === "school_admin" || role === "teacher";
  const canManage = role === "superadmin" || role === "admin" || role === "school_admin" || role === "teacher";
  const [selectedId, setSelectedId] = useState<number>();
  const [showCreate, setShowCreate] = useState(false);
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const users = useQuery({ queryKey: ["users"], queryFn: schoolApi.users, enabled: canManage });
  const create = useMutation({
    mutationFn: schoolApi.createClass,
    onSuccess: (schoolClass) => {
      client.invalidateQueries({ queryKey: ["classes"] });
      setSelectedId(schoolClass.id);
      setShowCreate(false);
    },
  });

  if (classes.isLoading) return <div className="p-8"><LoadingState /></div>;
  if (classes.isError) return <div className="p-8"><ErrorState onRetry={() => classes.refetch()} /></div>;

  const activeClassId = selectedId ?? classes.data?.[0]?.id;
  const selected = classes.data?.find((schoolClass) => schoolClass.id === activeClassId);
  return (
    <div className="p-8">
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Klassenverwaltung</h1>
          <p className="mt-2 text-sm text-gray-500">Klassen einstellen, Lehrkräfte zuweisen und Schüler:innen verwalten.</p>
        </div>
        {canCreate && <Button onClick={() => setShowCreate(true)}>Klasse anlegen</Button>}
      </div>

      {showCreate && (
        <ClassCreateForm
          pending={create.isPending}
          error={create.isError}
          onCancel={() => setShowCreate(false)}
          onSubmit={(data) => create.mutate(data)}
        />
      )}

      {!classes.data?.length ? (
        <EmptyState title="Noch keine Klassen" description="Lege zuerst eine Klasse an, um Mitglieder, Unterricht und Material zu organisieren." />
      ) : (
        <div className="grid gap-6 xl:grid-cols-[19rem_minmax(0,1fr)]">
          <aside className="overflow-hidden rounded-xl border border-gray-200 bg-white">
            <div className="border-b border-gray-200 px-4 py-3 text-xs font-semibold uppercase tracking-wide text-gray-500">Klassen</div>
            <div className="p-2">
              {classes.data.map((schoolClass) => (
                <button
                  key={schoolClass.id}
                  onClick={() => setSelectedId(schoolClass.id)}
                  className={`mb-1 flex w-full items-center justify-between rounded-lg px-3 py-3 text-left transition-colors ${schoolClass.id === activeClassId ? "bg-indigo-50 text-indigo-700" : "text-gray-700 hover:bg-gray-50"}`}
                >
                  <span><strong className="block text-sm">{schoolClass.name}</strong><span className="text-xs text-gray-500">{schoolClass.school_year}</span></span>
                  <span aria-hidden="true">›</span>
                </button>
              ))}
            </div>
          </aside>
          {selected && <ClassSettings key={selected.id} schoolClass={selected} users={users.data ?? []} canManageMembers={canManage} showRequests={canManage} />}
        </div>
      )}
    </div>
  );
}

function ClassSettings({ schoolClass, users, canManageMembers, showRequests }: { schoolClass: SchoolClass; users: Awaited<ReturnType<typeof schoolApi.users>>; canManageMembers: boolean; showRequests: boolean }) {
  const client = useQueryClient();
  const [name, setName] = useState(schoolClass.name);
  const [schoolYear, setSchoolYear] = useState(schoolClass.school_year);
  const [studentId, setStudentId] = useState("");
  const [teacherId, setTeacherId] = useState("");
  const [homeTeacher, setHomeTeacher] = useState(false);
  const members = useQuery({ queryKey: ["class-members", schoolClass.id], queryFn: () => schoolApi.classMembers(schoolClass.id) });
  const teachers = useQuery({ queryKey: ["class-teachers", schoolClass.id], queryFn: () => schoolApi.classTeachers(schoolClass.id) });
  const save = useMutation({ mutationFn: () => schoolApi.updateClass(schoolClass.id, { name, school_year: schoolYear }), onSuccess: () => client.invalidateQueries({ queryKey: ["classes"] }) });
  const addMember = useMutation({ mutationFn: (userId: number) => schoolApi.addClassMember(schoolClass.id, userId), onSuccess: () => { setStudentId(""); client.invalidateQueries({ queryKey: ["class-members", schoolClass.id] }); } });
  const removeMember = useMutation({ mutationFn: (userId: number) => schoolApi.removeClassMember(schoolClass.id, userId), onSuccess: () => client.invalidateQueries({ queryKey: ["class-members", schoolClass.id] }) });
  const addTeacher = useMutation({ mutationFn: (userId: number) => schoolApi.addClassTeacher(schoolClass.id, userId, homeTeacher), onSuccess: () => { setTeacherId(""); setHomeTeacher(false); client.invalidateQueries({ queryKey: ["class-teachers", schoolClass.id] }); } });
  const removeTeacher = useMutation({ mutationFn: (userId: number) => schoolApi.removeClassTeacher(schoolClass.id, userId), onSuccess: () => client.invalidateQueries({ queryKey: ["class-teachers", schoolClass.id] }) });
  const assignedStudentIds = new Set(members.data?.map((member) => member.user_id));
  const assignedTeacherIds = new Set(teachers.data?.map((teacher) => teacher.user_id));
  const availableStudents = users.filter((user) => user.role === "student" && !assignedStudentIds.has(user.id));
  const availableTeachers = users.filter((user) => user.role === "teacher" && !assignedTeacherIds.has(user.id));

  return (
    <section className="space-y-6">
      {showRequests && <JoinRequestPanel classId={schoolClass.id} />}
      <div className="rounded-xl border border-gray-200 bg-white p-5">
        <div className="mb-5 flex items-center justify-between"><div><h2 className="text-lg font-semibold text-gray-900">Klasse einstellen</h2><p className="mt-1 text-sm text-gray-500">Bezeichnung und Schuljahr werden direkt übernommen.</p></div></div>
        <div className="grid gap-4 md:grid-cols-2">
          <label className="text-sm font-medium text-gray-700">Klassenbezeichnung<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" value={name} onChange={(event) => setName(event.target.value)} /></label>
          <label className="text-sm font-medium text-gray-700">Schuljahr<input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" value={schoolYear} onChange={(event) => setSchoolYear(event.target.value)} /></label>
        </div>
        <div className="mt-4 flex items-center gap-3"><Button loading={save.isPending} onClick={() => save.mutate()}>Änderungen speichern</Button>{save.isError && <span className="text-sm text-red-600">Die Klasse konnte nicht gespeichert werden.</span>}</div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <AssignmentPanel
          title="Schüler:innen"
          description="Mitglieder dieser Klasse"
          empty="Noch keine Schüler:innen zugewiesen."
          entries={members.data ?? []}
          loading={members.isLoading}
          canManage={canManageMembers}
          options={availableStudents}
          value={studentId}
          onChange={setStudentId}
          onAdd={() => studentId && addMember.mutate(Number(studentId))}
          adding={addMember.isPending}
          onRemove={(userId) => removeMember.mutate(userId)}
          removing={removeMember.isPending}
        />
        <AssignmentPanel
          title="Lehrkräfte"
          description="Unterrichtende und Klassenleitung"
          empty="Noch keine Lehrkraft zugewiesen."
          entries={teachers.data ?? []}
          loading={teachers.isLoading}
          canManage={canManageMembers}
          options={availableTeachers}
          value={teacherId}
          onChange={setTeacherId}
          onAdd={() => teacherId && addTeacher.mutate(Number(teacherId))}
          adding={addTeacher.isPending}
          onRemove={(userId) => removeTeacher.mutate(userId)}
          removing={removeTeacher.isPending}
          extra={<label className="flex items-center gap-2 text-xs text-gray-600"><input type="checkbox" checked={homeTeacher} onChange={(event) => setHomeTeacher(event.target.checked)} /> Klassenleitung</label>}
        />
      </div>
    </section>
  );
}

type AssignmentEntry = { user_id: number; first_name: string; last_name: string; email: string; is_home_teacher?: boolean };
function AssignmentPanel({ title, description, empty, entries, options, value, onChange, onAdd, onRemove, loading, adding, removing, canManage, extra }: { title: string; description: string; empty: string; entries: AssignmentEntry[]; options: Awaited<ReturnType<typeof schoolApi.users>>; value: string; onChange: (value: string) => void; onAdd: () => void; onRemove: (id: number) => void; loading: boolean; adding: boolean; removing: boolean; canManage: boolean; extra?: React.ReactNode }) {
  return <div className="overflow-hidden rounded-xl border border-gray-200 bg-white"><div className="border-b border-gray-200 px-5 py-4"><h2 className="font-semibold text-gray-900">{title}</h2><p className="mt-1 text-sm text-gray-500">{description}</p></div>{canManage && <div className="border-b border-gray-100 bg-gray-50 p-4"><div className="flex gap-2"><select className="min-w-0 flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm" value={value} onChange={(event) => onChange(event.target.value)}><option value="">Person auswählen</option>{options.map((user) => <option key={user.id} value={user.id}>{user.last_name}, {user.first_name}</option>)}</select><Button size="sm" disabled={!value} loading={adding} onClick={onAdd}>Hinzufügen</Button></div>{extra && <div className="mt-2">{extra}</div>}</div>}<div>{loading ? <div className="p-5"><LoadingState label="Wird geladen" /></div> : entries.length ? entries.map((entry) => <div className="flex items-center gap-3 border-b border-gray-100 px-5 py-3 last:border-0" key={entry.user_id}><div className="grid size-8 place-items-center rounded-full bg-indigo-100 text-xs font-semibold text-indigo-700">{entry.first_name[0]}{entry.last_name[0]}</div><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium text-gray-900">{entry.first_name} {entry.last_name}{entry.is_home_teacher && <span className="ml-2 text-xs font-normal text-indigo-600">Klassenleitung</span>}</p><p className="truncate text-xs text-gray-500">{entry.email}</p></div>{canManage && <button className="text-sm font-medium text-red-600 hover:underline disabled:opacity-50" disabled={removing} onClick={() => onRemove(entry.user_id)}>Entfernen</button>}</div>) : <p className="p-5 text-sm text-gray-500">{empty}</p>}</div></div>;
}

function ClassCreateForm({ onSubmit, onCancel, pending, error }: { onSubmit: (data: Pick<SchoolClass, "name" | "school_year">) => void; onCancel: () => void; pending: boolean; error: boolean }) {
  const [name, setName] = useState(""); const [schoolYear, setSchoolYear] = useState(currentSchoolYear);
  return <form className="mb-6 grid gap-4 rounded-xl border border-indigo-200 bg-indigo-50 p-5 md:grid-cols-[1fr_1fr_auto]" onSubmit={(event) => { event.preventDefault(); onSubmit({ name, school_year: schoolYear }); }}><label className="text-sm font-medium text-gray-700">Klasse<input className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required placeholder="z. B. 7a" value={name} onChange={(event) => setName(event.target.value)} /></label><label className="text-sm font-medium text-gray-700">Schuljahr<input className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required value={schoolYear} onChange={(event) => setSchoolYear(event.target.value)} /></label><div className="flex items-end gap-2"><Button type="button" variant="ghost" onClick={onCancel}>Abbrechen</Button><Button loading={pending}>Anlegen</Button></div>{error && <p className="text-sm text-red-600 md:col-span-3">Die Klasse konnte nicht angelegt werden.</p>}</form>;
}

function JoinRequestPanel({ classId }: { classId: number }) {
  const client = useQueryClient();
  const requests = useQuery({ queryKey: ["join-requests", classId], queryFn: () => schoolApi.joinRequests(classId) });
  const refresh = () => {
    client.invalidateQueries({ queryKey: ["join-requests", classId] });
    client.invalidateQueries({ queryKey: ["class-members", classId] });
  };
  const approve = useMutation({ mutationFn: schoolApi.approveJoinRequest, onSuccess: refresh });
  const reject = useMutation({ mutationFn: schoolApi.rejectJoinRequest, onSuccess: refresh });

  if (requests.isLoading) return <div className="rounded-xl border border-gray-200 bg-white p-5"><LoadingState label="Anfragen werden geladen" /></div>;
  if (requests.isError) return null;
  if (!requests.data?.length) return null;

  return (
    <div className="overflow-hidden rounded-xl border border-amber-200 bg-amber-50">
      <div className="border-b border-amber-200 px-5 py-4">
        <h2 className="font-semibold text-gray-900">Beitrittsanfragen ({requests.data.length})</h2>
        <p className="mt-1 text-sm text-gray-500">Schüler:innen warten auf Freigabe für diese Klasse.</p>
      </div>
      <div>
        {requests.data.map((req) => (
          <div className="flex items-center gap-3 border-b border-amber-100 bg-white px-5 py-3 last:border-0" key={req.id}>
            <div className="grid size-8 place-items-center rounded-full bg-amber-100 text-xs font-semibold text-amber-800">
              {(req.student_first_name?.[0] ?? "?")}{(req.student_last_name?.[0] ?? "")}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-900">{req.student_first_name} {req.student_last_name}</p>
              <p className="truncate text-xs text-gray-500">{req.student_email}</p>
            </div>
            <button className="text-sm font-medium text-indigo-700 hover:underline disabled:opacity-50" disabled={approve.isPending || reject.isPending} onClick={() => approve.mutate(req.id)}>Freigeben</button>
            <button className="text-sm font-medium text-red-600 hover:underline disabled:opacity-50" disabled={approve.isPending || reject.isPending} onClick={() => reject.mutate(req.id)}>Ablehnen</button>
          </div>
        ))}
      </div>
      {(approve.isError || reject.isError) && <p className="bg-white px-5 py-3 text-sm text-red-600">Die Entscheidung konnte nicht gespeichert werden.</p>}
    </div>
  );
}
