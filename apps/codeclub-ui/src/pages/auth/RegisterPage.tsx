import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import axios from "axios";
import { schoolApi } from "../../api/school";
import { LogoMark } from "../../components/ui/Logo";

const tileLabel = "text-sm font-medium text-green-50";
const tileInput = "mt-1 w-full rounded-lg border border-white/40 bg-white/95 px-3 py-2 text-gray-900 placeholder:text-gray-500";
const tileSelect = "mt-1 w-full rounded-lg border border-white/40 bg-white/95 px-3 py-2 text-gray-900";

export function RegisterPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [schoolId, setSchoolId] = useState("");
  const [classId, setClassId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const schools = useQuery({ queryKey: ["public-schools"], queryFn: schoolApi.publicSchools });
  const classes = useQuery({
    queryKey: ["public-classes", schoolId],
    queryFn: () => schoolApi.publicClasses(Number(schoolId)),
    enabled: schoolId !== "",
  });

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!schoolId) {
      setError("Bitte wähle deine Schule aus.");
      return;
    }
    setPending(true);
    try {
      await schoolApi.register({
        email,
        password,
        first_name: firstName,
        last_name: lastName,
        school_id: Number(schoolId),
        ...(classId ? { requested_class_id: Number(classId) } : {}),
      });
      navigate("/login", { state: { registered: true } });
    } catch (err) {
      if (axios.isAxiosError(err)) {
        const status = err.response?.status;
        if (status === 409) setError("Diese E-Mail ist bereits registriert. Melde dich stattdessen an.");
        else if (status === 429) setError("Zu viele Versuche. Bitte warte einen Moment und versuche es erneut.");
        else setError("Registrierung fehlgeschlagen. Prüfe deine Angaben.");
      } else {
        setError("Registrierung fehlgeschlagen. Prüfe deine Angaben.");
      }
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="w-full max-w-md">
      <div className="rounded-2xl border border-white/15 bg-green-950/55 p-8 text-white shadow-xl backdrop-blur-xl dark:bg-black/55">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 w-fit">
            <LogoMark size={44} />
          </div>
          <h1 className="text-3xl text-white">Konto erstellen</h1>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <label className={tileLabel}>Vorname
              <input className={tileInput} required value={firstName} onChange={(e) => setFirstName(e.target.value)} autoComplete="given-name" />
            </label>
            <label className={tileLabel}>Nachname
              <input className={tileInput} required value={lastName} onChange={(e) => setLastName(e.target.value)} autoComplete="family-name" />
            </label>
          </div>
          <label className={tileLabel}>E-Mail
            <input className={tileInput} type="email" required placeholder="max.mustermann@schule.de" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
          </label>
          <label className={tileLabel}>Passwort (min. 8 Zeichen)
            <input className={tileInput} type="password" required minLength={8} placeholder="••••••••" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
          </label>
          <label className={tileLabel}>Schule
            <select className={tileSelect} required value={schoolId} onChange={(e) => { setSchoolId(e.target.value); setClassId(""); }}>
              <option value="">Schule wählen</option>
              {schools.data?.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </label>
          {schools.isError && <p className="text-sm text-red-200">Schulen konnten nicht geladen werden.</p>}
          {schoolId && (
            <label className={tileLabel}>Klasse <span className="font-normal text-green-100/70">(optional — Beitrittsanfrage)</span>
              <select className={tileSelect} value={classId} onChange={(e) => setClassId(e.target.value)}>
                <option value="">Später wählen</option>
                {classes.data?.map((c) => <option key={c.id} value={c.id}>{c.name} ({c.school_year})</option>)}
              </select>
            </label>
          )}

          {error && <div className="rounded-lg border border-red-200/30 bg-red-500/15 px-4 py-3 text-sm text-red-100">{error}</div>}

          <button type="submit" disabled={pending} style={{ backgroundImage: "none" }} className="mt-2 w-full rounded-lg border-0 bg-green-500 bg-none px-4 py-2.5 text-sm font-semibold text-green-950 shadow-none transition-colors hover:bg-green-500 disabled:opacity-50">
            {pending ? "Wird erstellt …" : "Registrieren"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-green-100/80">
          Bereits ein Konto? <Link className="font-semibold text-white hover:underline" to="/login">Anmelden</Link>
        </p>
      </div>
    </div>
  );
}
