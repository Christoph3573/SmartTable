import { Link } from "react-router-dom";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { schoolApi } from "../../api/school";
import { schoolConnectApi, scSubstitutionLabel } from "../../api/schoolconnect";
import { useAuthStore } from "../../store/authStore";
import { useSettingsStore } from "../../store/settingsStore";
import { ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { MembershipBanner } from "../../components/MembershipBanner";
import { formatDate } from "../../lib/format";

const channelLabel = (channel: { name?: string; type: string }) => channel.name || ({ direct: "Direktnachricht", class: "Klassenchat", group: "Gruppe" }[channel.type] ?? "Chat");

const typeLabels = { holiday: "Ferien", exam: "Prüfung", event: "Termin", other: "Sonstiges" };

const today = new Date().toISOString().slice(0, 10);
export function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const provider = useSettingsStore((s) => s.provider);
  const isExternal = provider === "schoolconnect";
  const substitutions = useQuery({
    queryKey: ["substitutions", "today"],
    queryFn: () => schoolApi.substitutions({ date_from: today, date_to: today }),
    enabled: !isExternal,
  });
  const externalSubs = useQuery({
    queryKey: ["schoolconnect", "vertretungsplan", today],
    queryFn: () => schoolConnectApi.vertretungsplan({ datum: today }),
    enabled: isExternal,
    retry: 1,
  });
  const subCount = isExternal
    ? (externalSubs.data?.eintraege.length ?? 0)
    : (substitutions.data?.length ?? 0);
  const events = useQuery({ queryKey: ["events", "upcoming"], queryFn: () => schoolApi.events({ start_date: today }) });
  const channels = useQuery({ queryKey: ["channels"], queryFn: schoolApi.channels });
  const unreadChannels = useMemo(() => (channels.data ?? []).filter((channel) => (channel.unread_count ?? 0) > 0), [channels.data]);
  const salutation = new Intl.DateTimeFormat("de-DE", { weekday: "long", day: "numeric", month: "long" }).format(new Date());
  const subsLoading = isExternal ? externalSubs.isLoading : substitutions.isLoading;
  const subsError = isExternal ? externalSubs.isError : substitutions.isError;
  const refetchSubs = () => { if (isExternal) externalSubs.refetch(); else substitutions.refetch(); events.refetch(); };
  return <div className="page"><PageHeader eyebrow={salutation} title={`Hallo, ${user?.first_name ?? ""}.`} />
    <MembershipBanner />
    {subsLoading || events.isLoading ? <LoadingState /> : subsError || events.isError ? <ErrorState onRetry={refetchSubs} /> : <><section className="dashboard-hero"><div><p className="eyebrow text-indigo-100">Dein Schultag</p><h2>Alles, was heute wichtig ist — an einem ruhigen Ort.</h2><p>{subCount ? `${subCount} Änderungen im Unterricht brauchen deine Aufmerksamkeit.` : "Dein Stundenplan ist heute unverändert."}{isExternal ? " (Quelle: SchoolConnect)" : ""}</p></div><Link to="/substitutions" className="hero-link">Tagesplan öffnen <span>→</span></Link></section>
    {unreadChannels.length > 0 && <section className="surface mt-5 overflow-hidden"><div className="flex items-center justify-between border-b border-gray-100 px-5 py-4"><h2>Ungelesene Nachrichten</h2><Link className="text-button" to="/chat">Zum Chat</Link></div>{unreadChannels.map((channel) => <Link className="data-row" key={channel.id} to="/chat"><span className="grid size-9 place-items-center rounded-lg bg-indigo-50 font-bold text-indigo-700">{channel.unread_count! > 9 ? "9+" : channel.unread_count}</span><div className="data-row-main"><strong>{channelLabel(channel)}</strong><p>{channel.unread_count} neue Nachricht{channel.unread_count === 1 ? "" : "en"}</p></div></Link>)}</section>}
    <div className="mt-5 grid gap-5 lg:grid-cols-[1.1fr_.9fr]"><section className="surface overflow-hidden"><div className="flex items-center justify-between border-b border-gray-100 px-5 py-4"><h2>Heute im Plan</h2><Link className="text-button" to="/substitutions">Alle anzeigen</Link></div>{isExternal ? (
      externalSubs.data?.eintraege.length ? externalSubs.data.eintraege.slice(0, 4).map((item, index) => {
        const label = scSubstitutionLabel(item);
        return <div className="data-row" key={index}><span className="grid size-9 place-items-center rounded-lg bg-indigo-50 font-bold text-indigo-700">{label.period}</span><div className="data-row-main"><strong>{label.title}</strong><p>{label.detail}</p></div><span className="pill blue">Neu</span></div>;
      }) : <div className="px-5 py-9 text-sm text-gray-500">Heute sind keine Vertretungen oder Raumwechsel eingetragen.</div>
    ) : substitutions.data?.length ? substitutions.data.map((item) => <div className="data-row" key={item.id}><span className="grid size-9 place-items-center rounded-lg bg-indigo-50 font-bold text-indigo-700">{item.period}.</span><div className="data-row-main"><strong>{item.type === "cancellation" ? "Unterricht entfällt" : "Unterricht wurde geändert"}</strong><p>{item.room ? `Raum ${item.room}` : "Details im Vertretungsplan"}{item.note ? ` · ${item.note}` : ""}</p></div><span className={`pill ${item.type === "cancellation" ? "red" : "blue"}`}>{item.type === "cancellation" ? "Entfall" : "Neu"}</span></div>) : <div className="px-5 py-9 text-sm text-gray-500">Heute sind keine Vertretungen oder Raumwechsel eingetragen.</div>}</section><section className="surface overflow-hidden"><div className="flex items-center justify-between border-b border-gray-100 px-5 py-4"><h2>Nächste Termine</h2><Link className="text-button" to="/calendar">Kalender</Link></div>{events.data?.slice().sort((a,b) => a.start_time.localeCompare(b.start_time)).slice(0,4).map((event) => <div className="data-row" key={event.id}><time className="w-12 font-mono text-xs text-gray-500">{formatDate(event.start_time)}</time><div className="data-row-main"><strong>{event.title}</strong><p>{event.all_day ? "Ganztägig" : `${new Date(event.start_time).toLocaleTimeString("de-DE", {hour:"2-digit",minute:"2-digit"})} Uhr`} · {typeLabels[event.type]}</p></div></div>) || null}{!events.data?.length && <div className="px-5 py-9 text-sm text-gray-500">Der Kalender ist für die nächsten Tage frei.</div>}</section></div></>}
  </div>;
}
