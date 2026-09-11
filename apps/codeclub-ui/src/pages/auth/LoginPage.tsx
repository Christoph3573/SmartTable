import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { Input } from "../../components/ui/Input";

export function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await login(email, password);
      navigate("/dashboard");
    } catch {
      setError("E-Mail oder Passwort ist falsch.");
    }
  };

  return (
    <div className="w-full max-w-sm">
      <div className="rounded-2xl border border-[#e1e3dc] bg-white p-8 shadow-[0_20px_50px_rgba(48,63,52,.08)]">
        <div className="mb-8 text-center">
          <span className="mx-auto mb-4 grid size-11 place-items-center rounded-xl bg-indigo-600 text-2xl font-bold text-white">S</span>
          <p className="eyebrow">SmartTable</p>
          <h1 className="text-3xl text-[#24283a]">Willkommen zurück</h1>
          <p className="mt-3 text-sm text-[#747a71]">Melde dich an, um deinen Schultag zu öffnen.</p>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            id="email"
            label="E-Mail"
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
            type="password"
            placeholder="••••••••"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
          />

          {error && (
            <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
              {error}
            </div>
          )}

          <Button type="submit" loading={isLoading} className="mt-2 w-full">
            Anmelden
          </Button>
        </form>
      </div>
    </div>
  );
}
