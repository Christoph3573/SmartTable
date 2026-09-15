import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import axios from "axios";
import { schoolApi } from "../../api/school";

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
      <div className="rounded-2xl border border-[#e1e3dc] bg-white p-8 shadow-[0_20px_50px_rgba(48,63,52,.08)]">
        <div className="mb-8 text-center">
          <span className="mx-auto mb-4 grid size-11 place-items-center rounded-xl bg-indigo-600 text-2xl font-bold text-white">S</span>
          <p className="eyebrow">SmartTable</p>
          <h1 className="text-3xl text-[#24283a]">Konto erstellen</h1>
          <p className="mt-3 text-sm text-[#747a71]">Registriere dich als Schüler:in deiner Schule. Dein Klassenbeitritt wird danach von einer Lehrkraft freigegeben.</p>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="text-sm font-medium text-gray-700">Vorname
              <input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={firstName} onChange={(e) => setFirstName(e.target.value)} autoComplete="given-name" />
            </label>
            <label className="text-sm font-medium text-gray-700">Nachname
              <input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" required value={lastName} onChange={(e) => setLastName(e.target.value)} autoComplete="family-name" />
            </label>
          </div>
          <label className="text-sm font-medium text-gray-700">E-Mail
            <input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" type="email" required placeholder="max.mustermann@schule.de" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
          </label>
          <label className="text-sm font-medium text-gray-700">Passwort (min. 8 Zeichen)
            <input className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" type="password" required minLength={8} placeholder="••••••••" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
          </label>
          <label className="text-sm font-medium text-gray-700">Schule
            <select className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" required value={schoolId} onChange={(e) => { setSchoolId(e.target.value); setClassId(""); }}>
              <option value="">Schule wählen</option>
              {schools.data?.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </label>
          {schools.isError && <p className="text-sm text-red-600">Schulen konnten nicht geladen werden.</p>}
          {schoolId && (
            <label className="text-sm font-medium text-gray-700">Klasse <span className="font-normal text-gray-400">(optional — Beitrittsanfrage)</span>
              <select className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2" value={classId} onChange={(e) => setClassId(e.target.value)}>
                <option value="">Später wählen</option>
                {classes.data?.map((c) => <option key={c.id} value={c.id}>{c.name} ({c.school_year})</option>)}
              </select>
            </label>
          )}

          {error && <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">{error}</div>}

          <button type="submit" disabled={pending} className="mt-2 w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-700 disabled:opacity-50">
            {pending ? "Wird erstellt …" : "Registrieren"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-gray-500">
          Bereits ein Konto? <Link className="font-semibold text-indigo-700 hover:underline" to="/login">Anmelden</Link>
        </p>
      </div>
    </div>
  );
}
