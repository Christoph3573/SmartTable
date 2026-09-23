import { useEffect, useRef, useState } from "react";
import { apiClient } from "../../api/client";
import { schoolApi } from "../../api/school";
import type { components } from "../../api/generated/types";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, PageHeader } from "../../components/ui/Page";
import { Markdown } from "../../components/ui/Markdown";
import { formatDate } from "../../lib/format";

type OpenCodeSession = components["schemas"]["OpenCodeSession"];
type OpenCodeMessage = components["schemas"]["OpenCodeMessage"];
type WsOpenCodeEvent = { type: "opencode"; session_id: number; message: OpenCodeMessage };

type ChatBubble = {
  id: string;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

/**
 * KI-Lernchat: Der Browser spricht ausschließlich mit dem Go-Backend
 * (POST/GET/DELETE /api/v1/integrations/opencode/sessions...), niemals
 * direkt mit `opencode serve`. Das Backend ist die Multi-Tenant-Grenze
 * (eigener Workspace /workspaces/<user_id> pro User, Ownership-Checks,
 * Schuldaten nur über Backend-MCP-Tools).
 *
 * Layout: links eine ChatGPT-artige Chat-Übersicht (Liste + „Neuer Chat"),
 * rechts der Verlauf mit Eingabezeile. Die Liste kommt vom Backend und
 * enthält ausschließlich die eigenen Sessions.
 */
export function AiChatPage() {
  const user = useAuthStore((s) => s.user);
  const [sessions, setSessions] = useState<OpenCodeSession[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null);
  const [messages, setMessages] = useState<ChatBubble[]>([]);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [backendState, setBackendState] = useState<"checking" | "online" | "offline">("checking");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const activeSession = sessions.find((s) => s.id === activeId) ?? null;

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const refreshStatus = () =>
    apiClient
      .get("/api/v1/integrations/opencode/status")
      .then((res) => setBackendState(res.data?.reachable ? "online" : "offline"))
      .catch(() => setBackendState("offline"));

  const refreshSessions = () =>
    apiClient
      .get<OpenCodeSession[]>("/api/v1/integrations/opencode/sessions")
      .then((res) => {
        setSessions(res.data);
        setActiveId((prev) => {
          if (prev !== null && res.data.some((s) => s.id === prev)) return prev;
          return res.data[0]?.id ?? null;
        });
      })
      .catch(() => {});

  useEffect(() => {
    refreshStatus();
    refreshSessions();
    const timer = setInterval(refreshStatus, 30000);
    return () => clearInterval(timer);
  }, []);

  // Verlauf laden bei Session-Wechsel.
  useEffect(() => {
    let cancelled = false;
    if (activeId === null) {
      const timer = setTimeout(() => {
        if (!cancelled) setMessages([]);
      }, 0);
      return () => {
        cancelled = true;
        clearTimeout(timer);
      };
    }
    const load = async () => {
      if (cancelled) return;
      setLoadingHistory(true);
      try {
        const res = await apiClient.get<OpenCodeMessage[]>(
          `/api/v1/integrations/opencode/sessions/${activeId}`
        );
        if (!cancelled) {
          setMessages(
            res.data.map((m) => ({
              id: String(m.id),
              role: m.role,
              content: m.content,
              created_at: m.created_at,
            }))
          );
        }
      } catch {
        if (!cancelled) setMessages([]);
      } finally {
        if (!cancelled) setLoadingHistory(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [activeId]);

  // WebSocket-Events (type "opencode") der aktiven Session einpflegen —
  // z. B. wenn die Antwort über einen anderen Tab ausgelöst wurde.
  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      let payload: WsOpenCodeEvent;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }
      if (payload?.type !== "opencode" || payload.session_id !== activeId || !payload.message) return;
      const incoming: ChatBubble = {
        id: String(payload.message.id),
        role: payload.message.role,
        content: payload.message.content,
        created_at: payload.message.created_at,
      };
      setMessages((prev) => (prev.some((m) => m.id === incoming.id) ? prev : [...prev, incoming]));
    };
    window.addEventListener("smarttable:opencode", onMessage as EventListener);
    return () => window.removeEventListener("smarttable:opencode", onMessage as EventListener);
  }, [activeId]);

  // Fallback-Polling: Verlauf alle 5s nachladen, solange die Seite offen ist
  // (deckt Antworten ohne WS-Event ab).
  useEffect(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    if (activeId === null) return;
    pollRef.current = setInterval(() => {
      if (document.hidden) return;
      apiClient
        .get<OpenCodeMessage[]>(`/api/v1/integrations/opencode/sessions/${activeId}`)
        .then((res) =>
          setMessages((prev) => {
            const known = new Set(prev.map((m) => m.id));
            const fresh = res.data
              .filter((m) => !known.has(String(m.id)))
              .map((m) => ({ id: String(m.id), role: m.role, content: m.content, created_at: m.created_at }));
            return fresh.length ? [...prev, ...fresh] : prev;
          })
        )
        .catch(() => {});
    }, 5000);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [activeId]);

  const push = (role: ChatBubble["role"], content: string) =>
    setMessages((prev) => [
      ...prev,
      { id: `${Date.now()}-${Math.random().toString(36).slice(2)}`, role, content, created_at: new Date().toISOString() },
    ]);

  const localAnswer = async (question: string): Promise<string> => {
    // Lokale Lernhilfe als Überbrückung, solange OpenCode offline ist:
    // fällige Vokabeln + offene Hausaufgaben der ersten Klasse zusammenfassen.
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
      return "Das opencode-Backend ist gerade offline, deshalb antworte ich lokal: " +
        (dueTotal > 0
          ? `Du hast ${dueTotal} fällige Vokabelkarten — starte am besten dort. `
          : "Deine Vokabeln sind auf Stand. ") +
        'Frag mich z. B. nach „Vokabeln“ oder „Hausaufgaben“, dann fasse ich deinen Lernstand zusammen.';
    } catch {
      return "Ich konnte deinen Lernstand gerade nicht laden. Versuch es gleich noch einmal — oder frag mich etwas Allgemeines zum Lernen.";
    }
  };

  const createSession = async (): Promise<number | null> => {
    try {
      const res = await apiClient.post<OpenCodeSession>("/api/v1/integrations/opencode/sessions", {
        title: "Lern-Chat",
      });
      setSessions((prev) => [res.data, ...prev]);
      setActiveId(res.data.id);
      setMessages([]);
      setBackendState("online");
      return res.data.id;
    } catch {
      await refreshStatus();
      return null;
    }
  };

  const deleteSession = async (id: number) => {
    try {
      await apiClient.delete(`/api/v1/integrations/opencode/sessions/${id}`);
    } catch {
      // Lokal trotzdem entfernen (z. B. Backend offline).
    }
    setSessions((prev) => {
      const rest = prev.filter((s) => s.id !== id);
      setActiveId((current) => (current === id ? (rest[0]?.id ?? null) : current));
      return rest;
    });
    if (activeId === id) setMessages([]);
  };

  const send = async (event: React.FormEvent) => {
    event.preventDefault();
    const question = text.trim();
    if (!question || sending) return;
    setText("");
    let sessionId = activeId;
    if (sessionId === null) {
      if (backendState === "online") {
        sessionId = await createSession();
        if (sessionId === null) {
          push("user", question);
          push("assistant", await localAnswer(question));
          return;
        }
      } else {
        push("user", question);
        push("assistant", await localAnswer(question));
        return;
      }
    }
    push("user", question);
    setSending(true);
    try {
      const res = await apiClient.post<OpenCodeMessage[]>(
        `/api/v1/integrations/opencode/sessions/${sessionId}/messages`,
        { content: question }
      );
      const reply = res.data.find((m) => m.role === "assistant");
      if (reply) {
        setMessages((prev) => {
          const withoutOptimisticUser = prev.slice(0, -1);
          const known = new Set(withoutOptimisticUser.map((m) => m.id));
          const fresh = res.data
            .filter((m) => !known.has(String(m.id)))
            .map((m) => ({ id: String(m.id), role: m.role, content: m.content, created_at: m.created_at }));
          return [...withoutOptimisticUser, ...fresh];
        });
      }
      refreshSessions();
    } catch {
      await refreshStatus();
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
      </PageHeader>

      <div className="grid min-h-[620px] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm md:grid-cols-[18rem_minmax(0,1fr)] dark:border-gray-700 dark:bg-gray-900">
        <aside className="flex flex-col border-b border-gray-200 bg-gray-50/70 md:border-b-0 md:border-r dark:border-gray-700 dark:bg-gray-950/40">
          <div className="flex items-center justify-between border-b border-gray-200 px-4 py-4 dark:border-gray-700">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Chats</h2>
            <button
              type="button"
              className="rounded-lg bg-indigo-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-indigo-700"
              onClick={() => void createSession()}
            >
              + Neu
            </button>
          </div>
          <nav className="flex-1 overflow-y-auto p-2">
            {sessions.map((session) => {
              const active = session.id === activeId;
              return (
                <div
                  key={session.id}
                  className={`group mb-1 flex items-center rounded-lg border ${
                    active
                      ? "border-indigo-200 bg-white shadow-sm dark:border-indigo-800 dark:bg-gray-800"
                      : "border-transparent hover:bg-white dark:hover:bg-gray-800"
                  }`}
                >
                  <button
                    type="button"
                    onClick={() => setActiveId(session.id)}
                    className="min-w-0 flex-1 px-3 py-2.5 text-left"
                    title={`${session.message_count} Nachrichten · Workspace ${session.workspace}`}
                  >
                    <span className={`block truncate text-sm font-medium ${active ? "text-indigo-900 dark:text-indigo-200" : "text-gray-700 dark:text-gray-200"}`}>
                      {session.title}
                    </span>
                    <span className="mt-0.5 block truncate text-xs text-gray-400">
                      {session.message_count} Nachricht{session.message_count === 1 ? "" : "en"}
                    </span>
                  </button>
                  <button
                    type="button"
                    aria-label={`Chat ${session.title} löschen`}
                    className="mr-1 rounded p-1 text-gray-400 opacity-0 transition-opacity hover:text-red-600 group-hover:opacity-100"
                    onClick={() => void deleteSession(session.id)}
                  >
                    ×
                  </button>
                </div>
              );
            })}
            {!sessions.length && (
              <p className="px-3 py-4 text-xs text-gray-500 dark:text-gray-400">
                {backendState === "online"
                  ? "Noch keine Chats — lege mit „+ Neu“ einen an."
                  : "OpenCode ist offline — deine Fragen werden lokal beantwortet (Vokabeln + Hausaufgaben)."}
              </p>
            )}
          </nav>
        </aside>

        <section className="flex min-h-[520px] flex-col">
          <header className="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-700">
            <div className="min-w-0">
              <h2 className="truncate text-base font-semibold text-gray-900 dark:text-white">
                {activeSession ? activeSession.title : "Neuer Chat"}
              </h2>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                Sessions sind pro Benutzer isoliert.
              </p>
            </div>
            {activeSession && (
              <span className="pill">{activeSession.message_count} Nachrichten</span>
            )}
          </header>

          <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
            {loadingHistory ? (
              <div className="flex items-center gap-2 self-start text-sm text-gray-500">
                <span className="loader" aria-hidden="true" /> Verlauf wird geladen …
              </div>
            ) : !messages.length ? (
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
                          : "rounded-bl-sm bg-white text-gray-800 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:text-gray-100 dark:ring-gray-700"
                      }`}
                    >
                      {own ? (
                        <p className="whitespace-pre-wrap break-words">{message.content}</p>
                      ) : (
                        <Markdown
                          content={message.content}
                          className="break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
                        />
                      )}
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

          <form className="flex gap-2 border-t border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-900" onSubmit={send}>
            <input
              aria-label="Nachricht an die Lern-KI"
              className="min-w-0 flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              placeholder={activeSession ? `Nachricht an ${activeSession.title} …` : "Frag mich etwas zum Lernen …"}
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
            <Button loading={sending} disabled={!text.trim()}>
              Senden
            </Button>
          </form>
        </section>
      </div>
      <p className="mt-3 text-xs text-gray-500 dark:text-gray-400">
        Sessions sind pro Benutzer isoliert (eigener Workspace auf dem Server). Die Lern-KI sieht Stundenplan,
        Vertretungen, Hausaufgaben, LehrplanPLUS und Vokabeln — aber nur deine eigenen Daten.
      </p>
    </div>
  );
}
