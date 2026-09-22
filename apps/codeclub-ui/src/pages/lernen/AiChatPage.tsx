import { useEffect, useRef, useState } from "react";
import { apiClient } from "../../api/client";
import { schoolApi } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

type AiMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

const STORAGE_KEY = "smarttable-ai-chat";
const OPENCODE_URL_KEY = "smarttable-opencode-url";

function loadMessages(): AiMessage[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as AiMessage[];
    return Array.isArray(parsed) ? parsed.slice(-100) : [];
  } catch {
    return [];
  }
}

/**
 * KI-Lernchat: schreibt gegen einen OpenAI-kompatiblen `opencode serve`
 * Sidecar (später eigener Compose-Service `opencode`, nur internes Netz,
 * erreichbar via Backend-Proxy POST /api/v1/integrations/opencode/chat).
 *
 * Stand jetzt: Der Sidecar läuft noch nicht — die Seite zeigt den
 * Verbindungsstatus, speichert den Verlauf lokal und antwortet mit einer
 * lokalen Lernhilfe (fällige Vokabeln + offene Hausaufgaben), bis das
 * opencode-Backend verdrahtet ist.
 */
export function AiChatPage() {
  const user = useAuthStore((s) => s.user);
  const [messages, setMessages] = useState<AiMessage[]>(loadMessages);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [backendState, setBackendState] = useState<"checking" | "online" | "offline">("checking");
  const [opencodeUrl, setOpencodeUrl] = useState(
    () => localStorage.getItem(OPENCODE_URL_KEY) ?? "http://opencode:8082"
  );
  const [showSettings, setShowSettings] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(messages.slice(-100)));
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    let cancelled = false;
    apiClient
      .get("/api/v1/integrations/opencode/status")
      .then(() => !cancelled && setBackendState("online"))
      .catch(() => !cancelled && setBackendState("offline"));
    return () => {
      cancelled = true;
    };
  }, []);

  const push = (role: AiMessage["role"], content: string) =>
    setMessages((prev) => [
      ...prev,
      { id: `${Date.now()}-${Math.random().toString(36).slice(2)}`, role, content, created_at: new Date().toISOString() },
    ]);

  const localAnswer = async (question: string): Promise<string> => {
    // Lokale Lernhilfe als Überbrückung: fällige Vokabeln + offene
    // Hausaufgaben der ersten Klasse zusammenfassen.
    const q = question.toLowerCase();
    try {
      const sets = await schoolApi.vocabSets();
      let dueTotal = 0;
      for (const set of sets.slice(0, 5)) {
        const cards = await schoolApi.vocabCards(set.id);
        dueTotal += cards.filter((c) => new Date(c.due_at) <= new Date()).length;
      }
      if (q.includes("vokabel")) {
        return dueTotal > 0
          ? `Aktuell sind ${dueTotal} Vokabelkarten fällig. Öffne „Lernen → Vokabeln" und starte die Abfrage — ich führe dich Karte für Karte durch.`
          : "Stark — keine Vokabelkarte ist gerade fällig. Lege neue Karten an oder wiederhole ein komplettes Set.";
      }
      if (q.includes("hausaufgabe") || q.includes("aufgabe")) {
        const classes = await schoolApi.classes();
        if (!classes.length) return "Du bist noch keiner Klasse zugeordnet — Hausaufgaben erscheinen, sobald du einer Klasse beitrittst.";
        const homework = await schoolApi.homework(classes[0].id);
        if (!homework.length) return "Für deine Klasse stehen aktuell keine Hausaufgaben an. Gute Gelegenheit, Vokabeln zu wiederholen!";
        const next = homework.slice().sort((a, b) => a.due_date.localeCompare(b.due_date))[0];
        return `Nächste Aufgabe: „${next.title}" (fällig ${next.due_date})${next.description ? ` — ${next.description}` : ""}. Sag mir, wobei du festhängst, dann gehen wir es Schritt für Schritt durch.`;
      }
      return `Gute Frage! Das opencode-Backend ist noch nicht verdrahtet (Status: ${backendState === "online" ? "online" : "offline"}), deshalb antworte ich lokal: ` +
        (dueTotal > 0
          ? `Du hast ${dueTotal} fällige Vokabelkarten — starte am besten dort. `
          : "Deine Vokabeln sind auf Stand. ") +
        'Frag mich z. B. nach „Vokabeln“ oder „Hausaufgaben“, dann fasse ich deinen Lernstand zusammen.';
    } catch {
      return "Ich konnte deinen Lernstand gerade nicht laden. Versuch es gleich noch einmal — oder frag mich etwas Allgemeines zum Lernen.";
    }
  };

  const send = async (event: React.FormEvent) => {
    event.preventDefault();
    const question = text.trim();
    if (!question || sending) return;
    setText("");
    push("user", question);
    setSending(true);
    try {
      if (backendState === "online") {
        const res = await apiClient.post<{ reply: string }>("/api/v1/integrations/opencode/chat", {
          message: question,
          history: messages.slice(-10).map((m) => ({ role: m.role, content: m.content })),
        });
        push("assistant", res.data.reply);
      } else {
        push("assistant", await localAnswer(question));
      }
    } catch {
      push("assistant", await localAnswer(question));
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="page">
      <PageHeader eyebrow="Lern-Bereich · KI" title="KI-Chat">
        <span className={`pill ${backendState === "online" ? "blue" : "amber"}`}>
          {backendState === "checking" ? "Verbinde …" : backendState === "online" ? "opencode verbunden" : "Lokaler Modus"}
        </span>
        <Button variant="secondary" onClick={() => setShowSettings((v) => !v)}>
          {showSettings ? "Schließen" : "Anbindung"}
        </Button>
      </PageHeader>

      {showSettings && (
        <section className="surface mb-5 p-5">
          <h2 className="mb-2">opencode-Anbindung (später)</h2>
          <p className="mb-3 text-sm text-gray-500">
            Der KI-Chat spricht später mit <code>opencode serve</code> als eigenem Compose-Service
            (<code>opencode:8082</code>, nur internes Netz) über den Backend-Proxy
            <code> POST /api/v1/integrations/opencode/chat</code>. Bis dahin läuft der lokale Modus.
          </p>
          <label className="block text-xs text-gray-600">
            opencode-URL (wird gespeichert, aktuell nur Anzeige)
            <input
              className="mt-1 w-full max-w-md rounded-lg border border-gray-300 p-2 text-sm"
              value={opencodeUrl}
              onChange={(e) => {
                setOpencodeUrl(e.target.value);
                localStorage.setItem(OPENCODE_URL_KEY, e.target.value);
              }}
            />
          </label>
          <div className="mt-3 flex gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setBackendState("checking");
                apiClient
                  .get("/api/v1/integrations/opencode/status")
                  .then(() => setBackendState("online"))
                  .catch(() => setBackendState("offline"));
              }}
            >
              Verbindung prüfen
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setMessages([]);
                localStorage.removeItem(STORAGE_KEY);
              }}
            >
              Verlauf löschen
            </Button>
          </div>
        </section>
      )}

      <section className="surface flex min-h-[540px] flex-col overflow-hidden">
        <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
          {!messages.length ? (
            <EmptyState
              title={`Hallo ${user?.first_name ?? ""} — frag mich was!`}
              description="Z. B. „Welche Vokabeln sind fällig?“, „Was sind meine nächsten Hausaufgaben?“ oder „Erklär mir das Passiv im Englischen.“"
            />
          ) : (
            messages.map((message) => {
              const own = message.role === "user";
              return (
                <div className={`max-w-[80%] ${own ? "self-end" : "self-start"}`} key={message.id}>
                  <p className={`mb-1 px-1 text-[11px] font-medium ${own ? "text-right text-indigo-700" : "text-gray-500"}`}>
                    {own ? "Du" : "Lern-KI"}
                  </p>
                  <div
                    className={`rounded-2xl px-4 py-3 text-sm leading-6 ${
                      own
                        ? "rounded-br-sm bg-indigo-600 text-white"
                        : "rounded-bl-sm bg-white text-gray-800 shadow-sm ring-1 ring-gray-200"
                    }`}
                  >
                    <p className="whitespace-pre-wrap break-words">{message.content}</p>
                    <time className={`mt-1.5 block text-[.65rem] ${own ? "text-indigo-100" : "text-gray-400"}`}>
                      {formatDate(message.created_at, { hour: "2-digit", minute: "2-digit" })}
                    </time>
                  </div>
                </div>
              );
            })
          )}
          {sending && (
            <div className="flex items-center gap-2 self-start text-sm text-gray-500">
              <span className="loader" aria-hidden="true" /> Lern-KI schreibt …
            </div>
          )}
          <div ref={bottomRef} />
        </div>
        <form className="flex gap-2 border-t border-gray-200 bg-white p-3" onSubmit={send}>
          <input
            aria-label="Nachricht an die Lern-KI"
            className="min-w-0 flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm"
            placeholder="Frag mich etwas zum Lernen …"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          <Button loading={sending} disabled={!text.trim()}>
            Senden
          </Button>
        </form>
      </section>
    </div>
  );
}
