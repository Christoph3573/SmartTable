import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi } from "../../api/school";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui/Page";

export function SchoolsPage() {
  const client = useQueryClient();
  const [name, setName] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const schools = useQuery({ queryKey: ["schools"], queryFn: schoolApi.schools });
  const create = useMutation({
    mutationFn: schoolApi.createSchool,
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["schools"] });
      setName("");
      setShowCreate(false);
    },
  });

  if (schools.isLoading) return <div className="p-8"><LoadingState /></div>;
  if (schools.isError) return <div className="p-8"><ErrorState onRetry={() => schools.refetch()} /></div>;

  return (
    <div className="p-8">
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Schulen</h1>
          <p className="mt-2 text-sm text-gray-500">Schulen anlegen — danach Schul-Admins über die Benutzerverwaltung zuordnen.</p>
        </div>
        <Button onClick={() => setShowCreate(true)}>Schule anlegen</Button>
      </div>

      {showCreate && (
        <form
          className="mb-6 flex flex-wrap items-end gap-3 rounded-xl border border-indigo-200 bg-indigo-50 p-5"
          onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate(name.trim()); }}
        >
          <label className="min-w-60 flex-1 text-sm font-medium text-gray-700">Schulname
            <input className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required autoFocus placeholder="z. B. Gymnasium Musterstadt" value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>Abbrechen</Button>
          <Button loading={create.isPending}>Anlegen</Button>
          {create.isError && <p className="w-full text-sm text-red-600">Die Schule konnte nicht angelegt werden (Name ggf. schon vergeben).</p>}
        </form>
      )}

      {!schools.data?.length ? (
        <EmptyState title="Noch keine Schulen" description="Lege die erste Schule an." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {schools.data.map((s) => (
            <div key={s.id} className="rounded-xl border border-gray-200 bg-white p-5">
              <h2 className="text-lg font-semibold text-gray-900">{s.name}</h2>
              <p className="mt-1 text-xs text-gray-500">ID {s.id}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
