import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { schoolApi } from "../api/school";
import { useAuthStore } from "../store/authStore";
import { useSettingsStore } from "../store/settingsStore";

export function MembershipBanner() {
  const user = useAuthStore((s) => s.user);
  const provider = useSettingsStore((s) => s.provider);
  const membership = useQuery({
    queryKey: ["membership"],
    queryFn: schoolApi.membership,
    enabled: user?.role === "student",
    staleTime: 30_000,
  });

  // Bei SchoolConnect kommen Klassen/Kurse aus dem Schülerportal — der
  // SmartTable-Beitritts-Hinweis wäre irreführend und wird ausgeblendet.
  if (provider === "schoolconnect") return null;

  if (user?.role !== "student" || membership.isLoading || membership.isError || !membership.data) return null;

  const hasClasses = membership.data.classes.length > 0;
  if (hasClasses) return null;

  const pending = membership.data.pending;

  return (
    <div className="mb-5 rounded-xl border border-amber-200 bg-amber-50 px-5 py-4">
      {pending.length > 0 ? (
        <>
          <p className="text-sm font-semibold text-amber-900">Beitrittsanfrage läuft</p>
          <p className="mt-1 text-sm text-amber-800">
            Deine Anfrage wartet auf Freigabe durch eine Lehrkraft. Sobald sie bestätigt ist, siehst du hier deinen Stundenplan und deine Hausaufgaben.
          </p>
        </>
      ) : (
        <>
          <p className="text-sm font-semibold text-amber-900">Noch keiner Klasse zugeordnet</p>
          <p className="mt-1 text-sm text-amber-800">
            Tritt deiner Klasse bei, um Vertretungen, Hausaufgaben und Dateien zu sehen.{" "}
            <Link className="font-semibold underline" to="/join">Jetzt Klasse wählen →</Link>
          </p>
        </>
      )}
    </div>
  );
}
