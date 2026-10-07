import { useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import axios from "axios";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { Input } from "../../components/ui/Input";

export function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation() as { state?: { registered?: boolean } };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await login(email, password);
      navigate("/dashboard");
    } catch (err) {
      if (axios.isAxiosError(err)) {
        const status = err.response?.status;
        if (status === 429) setError("Zu viele Versuche. Bitte warte einen Moment und versuche es erneut.");
        else if (status === 401) setError("E-Mail oder Passwort ist falsch.");
        else setError("Anmeldung fehlgeschlagen. Bitte versuche es erneut.");
      } else {
        setError("E-Mail oder Passwort ist falsch.");
      }
    }
  };

  return (
    <div className="w-full max-w-sm">
      <div className="rounded-2xl border border-white/15 bg-green-950/55 p-6 text-white shadow-xl backdrop-blur-xl md:p-8 dark:bg-black/55">
        <h1 className="sr-only">Anmelden</h1>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            id="email"
            label="E-Mail"
            labelClassName="text-green-50"
            className="border-white/40 bg-white/95 text-gray-900 placeholder:text-gray-500"
            type="email"
            placeholder="max.mustermann@schule.de"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            autoFocus
          />
          <Input
            id="password"
            label="Passwort"
            labelClassName="text-green-50"
            className="border-white/40 bg-white/95 text-gray-900 placeholder:text-gray-500"
            type="password"
            placeholder="••••••••"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
          />

          {location.state?.registered && !error && (
            <div className="mb-4 rounded-lg border border-green-200/30 bg-green-400/15 px-4 py-3 text-sm text-green-100">
              Konto erstellt. Melde dich jetzt an — deine Klassenanfrage wartet auf Freigabe.
            </div>
          )}

          {error && (
            <div className="rounded-lg border border-red-200/30 bg-red-500/15 px-4 py-3 text-sm text-red-100">
              {error}
            </div>
          )}

          <Button type="submit" loading={isLoading} size="sm" style={{ backgroundImage: "none" }} className="mt-2 w-full border-0 bg-green-500 bg-none font-semibold text-green-950 shadow-none hover:bg-green-500 dark:bg-green-700 dark:text-white dark:hover:bg-green-700">
            Anmelden
          </Button>
        </form>

        <p className="mt-6 text-center text-sm text-green-100/80">
          Noch kein Konto? <Link className="font-semibold text-white hover:underline" to="/register">Konto erstellen</Link>
        </p>
      </div>
    </div>
  );
}
