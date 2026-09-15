import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import axios from "axios";
import { schoolApi } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui/Page";
import { Button } from "../../components/ui/Button";

export function JoinClassPage() {
  const client = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const schoolId = user?.school_id;
  const [error, setError] = useState<string | null>(null);

  const membership = useQuery({ queryKey: ["membership"], queryFn: schoolApi.membership });
  const classes = useQuery({
    queryKey: ["public-classes", schoolId],
    queryFn: () => schoolApi.publicClasses(schoolId!),
    enabled: schoolId != null,
  });

  const request = useMutation({
    mutationFn: schoolApi.requestJoin,
    onSuccess: () => {
      setError(null);
      client.invalidateQueries({ queryKey: ["membership"] });
    },
    onError: (err) => {
      if (axios.isAxiosError(err) && err.response?.status === 409) setError("Du bist bereits Mitglied dieser Klasse oder hast schon angefragt.");
      else setError("Anfrage konnte nicht gestellt werden.");
    },
  });

  if (membership.isLoading) return <div className="p-8"><LoadingState /></div>;
  if (membership.isError) return <div className="p-8"><ErrorState onRetry={() => membership.refetch()} /></div>;

  const pendingIds = new Set(membership.data?.pending.map((p) => p.class_id));
  const memberIds = new Set(membership.data?.classes.map((c) => c.id));

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold text-gray-900">Klasse beitreten</h1>
        <p className="mt-2 text-sm text-gray-500">
          {membership.data?.school?.name ? `Deine Schule: ${membership.data.school.name}. ` : ""}Wähle deine Klasse — eine Lehrkraft gibt die Anfrage frei.
        </p>
      </div>

      {error && <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">{error}</div>}

      {!schoolId ? (
        <EmptyState title="Keine Schule zugeordnet" description="Dein Konto hat keine Schule. Wende dich an deine Lehrkraft." />
      ) : classes.isLoading ? (
        <LoadingState />
      ) : classes.isError ? (
        <ErrorState onRetry={() => classes.refetch()} />
      ) : !classes.data?.length ? (
        <EmptyState title="Keine Klassen" description="Für deine Schule sind noch keine Klassen angelegt." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {classes.data.map((c) => {
            const isMember = memberIds.has(c.id);
            const isPending = pendingIds.has(c.id);
            return (
              <div key={c.id} className="rounded-xl border border-gray-200 bg-white p-5">
                <h2 className="text-lg font-semibold text-gray-900">{c.name}</h2>
                <p className="mt-1 text-sm text-gray-500">Schuljahr {c.school_year}</p>
                <div className="mt-4">
                  {isMember ? (
                    <span className="rounded-full bg-green-50 px-2.5 py-1 text-xs font-medium text-green-700">Mitglied</span>
                  ) : isPending ? (
                    <span className="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700">Wartet auf Freigabe</span>
                  ) : (
                    <Button size="sm" loading={request.isPending} onClick={() => request.mutate(c.id)}>Beitreten anfragen</Button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <p className="mt-6 text-sm text-gray-500"><Link className="font-medium text-indigo-700 hover:underline" to="/dashboard">Zurück zum Dashboard</Link></p>
    </div>
  );
}
