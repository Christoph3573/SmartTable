import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  schoolApi,
  type CreateVocabCardInput,
  type CreateVocabSetInput,
  type VocabSet,
} from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";

const LANGS = [
  { value: "de", label: "Deutsch" },
  { value: "en", label: "Englisch" },
  { value: "fr", label: "Französisch" },
  { value: "es", label: "Spanisch" },
  { value: "it", label: "Italienisch" },
  { value: "la", label: "Latein" },
];

type QuizState = {
  order: number[];
  index: number;
  revealed: boolean;
  direction: "forward" | "backward";
  done: number;
};

/**
 * Vokabeln: Sets anlegen (optional einer Klasse teilen), Karteikarten
 * pflegen und per Leitner-Abfrage lernen (gewusst → Box+1, sonst Box 1).
 */
export function VocabPage() {
  const client = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const [setId, setSetId] = useState<number>();
  const [createSetOpen, setCreateSetOpen] = useState(false);
  const [quiz, setQuiz] = useState<QuizState | null>(null);

  const sets = useQuery({ queryKey: ["vocab-sets"], queryFn: schoolApi.vocabSets });
  const activeSet = sets.data?.find((s) => s.id === setId) ?? sets.data?.[0];
  const cards = useQuery({
    queryKey: ["vocab-cards", activeSet?.id],
    queryFn: () => schoolApi.vocabCards(activeSet!.id),
    enabled: Boolean(activeSet),
  });

  const removeSet = useMutation({
    mutationFn: schoolApi.deleteVocabSet,
    onSuccess: () => {
      setSetId(undefined);
      setQuiz(null);
      client.invalidateQueries({ queryKey: ["vocab-sets"] });
    },
  });

  const dueCount = useMemo(
    () => (cards.data ?? []).filter((c) => new Date(c.due_at) <= new Date()).length,
    [cards.data]
  );

  const startQuiz = (direction: "forward" | "backward", onlyDue: boolean) => {
    const pool = (cards.data ?? []).filter(
      (c) => !onlyDue || new Date(c.due_at) <= new Date()
    );
    if (!pool.length) return;
    const order = pool.map((c) => c.id).sort(() => Math.random() - 0.5);
    setQuiz({ order, index: 0, revealed: false, direction, done: 0 });
  };

  return (
    <div className="page">
      <PageHeader eyebrow="Lern-Bereich" title="Vokabeln">
        <Button onClick={() => setCreateSetOpen(true)}>Neues Set</Button>
      </PageHeader>

      {sets.isLoading ? (
        <LoadingState label="Vokabelsets werden geladen" />
      ) : sets.isError ? (
        <ErrorState onRetry={() => sets.refetch()} />
      ) : !sets.data?.length ? (
        <EmptyState title="Noch keine Vokabelsets" description="Lege dein erstes Set an — z. B. Englisch, Unit 3.">
          <Button className="mt-3" onClick={() => setCreateSetOpen(true)}>Set anlegen</Button>
        </EmptyState>
      ) : (
        <div className="grid gap-5 lg:grid-cols-[19rem_minmax(0,1fr)]">
          <aside className="surface overflow-hidden">
            <div className="border-b border-gray-100 px-5 py-4"><h2>Deine Sets</h2></div>
            <nav className="p-2">
              {sets.data.map((set) => (
                <button
                  key={set.id}
                  onClick={() => { setSetId(set.id); setQuiz(null); }}
                  className={`mb-1 w-full rounded-lg border px-3 py-3 text-left transition-colors ${
                    set.id === activeSet?.id
                      ? "border-indigo-200 bg-white text-indigo-950 shadow-sm"
                      : "border-transparent text-gray-700 hover:bg-white"
                  }`}
                >
                  <span className="block truncate text-sm font-semibold">{set.title}</span>
                  <span className="mt-1 block text-xs text-gray-500">
                    {set.source_lang} → {set.target_lang} · {set.card_count} Karten
                    {set.class_id ? " · geteilt" : ""}
                    {set.owner_id !== user?.id ? " · von Klasse" : ""}
                  </span>
                </button>
              ))}
            </nav>
          </aside>

          <section className="min-w-0">
            {quiz && activeSet ? (
              <QuizView
                set={activeSet}
                quiz={quiz}
                onExit={() => {
                  setQuiz(null);
                  client.invalidateQueries({ queryKey: ["vocab-cards", activeSet.id] });
                }}
                onAdvance={(next) => setQuiz(next)}
              />
            ) : (
              <SetDetail
                set={activeSet!}
                dueCount={dueCount}
                onStartQuiz={startQuiz}
                onDelete={() => activeSet && removeSet.mutate(activeSet.id)}
                deleting={removeSet.isPending}
              />
            )}
          </section>
        </div>
      )}

      {createSetOpen && (
        <CreateSetDialog
          pending={false}
          error={false}
          onClose={() => setCreateSetOpen(false)}
          onSubmit={async (data) => {
            const created = await schoolApi.createVocabSet(data);
            client.invalidateQueries({ queryKey: ["vocab-sets"] });
            setSetId(created.id);
            setCreateSetOpen(false);
          }}
        />
      )}
    </div>
  );
}

function SetDetail({
  set,
  dueCount,
  onStartQuiz,
  onDelete,
  deleting,
}: {
  set: VocabSet;
  dueCount: number;
  onStartQuiz: (direction: "forward" | "backward", onlyDue: boolean) => void;
  onDelete: () => void;
  deleting: boolean;
}) {
  const client = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const isOwner = set.owner_id === user?.id;
  const [cardForm, setCardForm] = useState({ front: "", back: "", hint: "" });
  const [editingId, setEditingId] = useState<number>();

  const cards = useQuery({
    queryKey: ["vocab-cards", set.id],
    queryFn: () => schoolApi.vocabCards(set.id),
  });

  const addCard = useMutation({
    mutationFn: (data: CreateVocabCardInput) => schoolApi.createVocabCard(set.id, data),
    onSuccess: () => {
      setCardForm({ front: "", back: "", hint: "" });
      client.invalidateQueries({ queryKey: ["vocab-cards", set.id] });
      client.invalidateQueries({ queryKey: ["vocab-sets"] });
    },
  });
  const saveCard = useMutation({
    mutationFn: ({ id, front, back, hint }: { id: number; front: string; back: string; hint?: string }) =>
      schoolApi.updateVocabCard(id, { front, back, hint: hint || undefined }),
    onSuccess: () => {
      setEditingId(undefined);
      client.invalidateQueries({ queryKey: ["vocab-cards", set.id] });
    },
  });
  const removeCard = useMutation({
    mutationFn: schoolApi.deleteVocabCard,
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["vocab-cards", set.id] });
      client.invalidateQueries({ queryKey: ["vocab-sets"] });
    },
  });

  return (
    <div className="flex flex-col gap-5">
      <section className="surface overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4">
          <div>
            <h2>{set.title}</h2>
            <p className="mt-1 text-xs text-gray-500">
              {set.source_lang} → {set.target_lang}
              {set.description ? ` · ${set.description}` : ""}
              {dueCount > 0 ? ` · ${dueCount} fällig` : " · nichts fällig — stark!"}
            </p>
          </div>
          {isOwner && (
            <Button size="sm" variant="ghost" loading={deleting} onClick={onDelete}>
              Set löschen
            </Button>
          )}
        </div>
        <div className="flex flex-wrap gap-2 p-5">
          <Button size="sm" disabled={!cards.data?.length} onClick={() => onStartQuiz("forward", true)}>
            Fällige abfragen{dueCount > 0 ? ` (${dueCount})` : ""}
          </Button>
          <Button size="sm" variant="secondary" disabled={!cards.data?.length} onClick={() => onStartQuiz("forward", false)}>
            Alle abfragen
          </Button>
          <Button size="sm" variant="secondary" disabled={!cards.data?.length} onClick={() => onStartQuiz("backward", false)}>
            Umgekehrt abfragen
          </Button>
        </div>
      </section>

      {isOwner && (
        <section className="surface p-5">
          <h2 className="mb-3">Karte hinzufügen</h2>
          <form
            className="grid gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]"
            onSubmit={(e) => {
              e.preventDefault();
              if (cardForm.front.trim() && cardForm.back.trim()) {
                addCard.mutate({
                  front: cardForm.front.trim(),
                  back: cardForm.back.trim(),
                  hint: cardForm.hint.trim() || undefined,
                });
              }
            }}
          >
            <input aria-label="Vorderseite" className="rounded-lg border border-gray-300 p-2 text-sm" placeholder={`Wort (${set.source_lang})`} value={cardForm.front} onChange={(e) => setCardForm({ ...cardForm, front: e.target.value })} />
            <input aria-label="Rückseite" className="rounded-lg border border-gray-300 p-2 text-sm" placeholder={`Übersetzung (${set.target_lang})`} value={cardForm.back} onChange={(e) => setCardForm({ ...cardForm, back: e.target.value })} />
            <input aria-label="Hinweis" className="rounded-lg border border-gray-300 p-2 text-sm" placeholder="Hinweis (optional)" value={cardForm.hint} onChange={(e) => setCardForm({ ...cardForm, hint: e.target.value })} />
            <Button loading={addCard.isPending}>Hinzufügen</Button>
          </form>
          {addCard.isError && <p className="mt-2 text-xs text-red-600">Karte konnte nicht gespeichert werden.</p>}
        </section>
      )}

      <section className="surface overflow-hidden">
        <div className="border-b border-gray-100 px-5 py-4"><h2>Karten ({cards.data?.length ?? 0})</h2></div>
        {cards.isLoading ? (
          <LoadingState label="Karten werden geladen" />
        ) : cards.isError ? (
          <ErrorState onRetry={() => cards.refetch()} />
        ) : !cards.data?.length ? (
          <EmptyState title="Noch keine Karten" description={isOwner ? "Füge oben die erste Karte hinzu." : "Dieses Set hat noch keine Karten."} />
        ) : (
          cards.data.map((card) => (
            <div className="data-row" key={card.id}>
              <span className={`grid size-9 place-items-center rounded-lg font-mono text-xs ${new Date(card.due_at) <= new Date() ? "bg-amber-50 text-amber-700" : "bg-green-50 text-green-700"}`}>
                B{card.box}
              </span>
              <div className="data-row-main">
                {editingId === card.id ? (
                  <CardEditForm
                    front={card.front}
                    back={card.back}
                    hint={card.hint ?? ""}
                    pending={saveCard.isPending}
                    onCancel={() => setEditingId(undefined)}
                    onSave={(front, back, hint) => saveCard.mutate({ id: card.id, front, back, hint })}
                  />
                ) : (
                  <>
                    <strong>{card.front} <span className="font-normal text-gray-400">→</span> {card.back}</strong>
                    <p>{card.hint ? `Hinweis: ${card.hint} · ` : ""}Fällig: {new Date(card.due_at).toLocaleDateString("de-DE", { day: "2-digit", month: "short" })}</p>
                  </>
                )}
              </div>
              {isOwner && editingId !== card.id && (
                <div className="flex gap-2">
                  <button className="text-button" onClick={() => setEditingId(card.id)}>Bearbeiten</button>
                  <button className="text-button text-red-600" onClick={() => removeCard.mutate(card.id)}>Löschen</button>
                </div>
              )}
            </div>
          ))
        )}
      </section>
    </div>
  );
}

function CardEditForm({
  front,
  back,
  hint,
  pending,
  onCancel,
  onSave,
}: {
  front: string;
  back: string;
  hint: string;
  pending: boolean;
  onCancel: () => void;
  onSave: (front: string, back: string, hint: string) => void;
}) {
  const [form, setForm] = useState({ front, back, hint });
  return (
    <form
      className="flex flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        onSave(form.front.trim(), form.back.trim(), form.hint.trim());
      }}
    >
      <input className="w-32 rounded-lg border border-gray-300 p-1.5 text-sm" value={form.front} onChange={(e) => setForm({ ...form, front: e.target.value })} required />
      <input className="w-32 rounded-lg border border-gray-300 p-1.5 text-sm" value={form.back} onChange={(e) => setForm({ ...form, back: e.target.value })} required />
      <input className="w-32 rounded-lg border border-gray-300 p-1.5 text-sm" placeholder="Hinweis" value={form.hint} onChange={(e) => setForm({ ...form, hint: e.target.value })} />
      <Button size="sm" loading={pending}>Speichern</Button>
      <button type="button" className="text-button" onClick={onCancel}>Abbrechen</button>
    </form>
  );
}

function QuizView({
  set,
  quiz,
  onExit,
  onAdvance,
}: {
  set: VocabSet;
  quiz: QuizState;
  onExit: () => void;
  onAdvance: (next: QuizState | null) => void;
}) {
  const client = useQueryClient();
  const cards = useQuery({
    queryKey: ["vocab-cards", set.id],
    queryFn: () => schoolApi.vocabCards(set.id),
  });
  const [grading, setGrading] = useState(false);

  const byId = useMemo(() => new Map((cards.data ?? []).map((c) => [c.id, c])), [cards.data]);
  const current = byId.get(quiz.order[quiz.index]);
  const total = quiz.order.length;

  const grade = async (known: boolean) => {
    if (!current || grading) return;
    setGrading(true);
    try {
      await schoolApi.gradeVocabCard(current.id, known);
      const nextIndex = quiz.index + 1;
      if (nextIndex >= total) {
        client.invalidateQueries({ queryKey: ["vocab-cards", set.id] });
        onAdvance(null);
        onExit();
      } else {
        onAdvance({ ...quiz, index: nextIndex, revealed: false, done: quiz.done + 1 });
      }
    } finally {
      setGrading(false);
    }
  };

  if (cards.isLoading) return <LoadingState label="Abfrage wird vorbereitet" />;
  if (!current) return <EmptyState title="Abfrage beendet" description="Alle Karten dieser Runde sind durch." />;

  const question = quiz.direction === "forward" ? current.front : current.back;
  const answer = quiz.direction === "forward" ? current.back : current.front;
  const questionLang = quiz.direction === "forward" ? set.source_lang : set.target_lang;
  const answerLang = quiz.direction === "forward" ? set.target_lang : set.source_lang;

  return (
    <section className="surface overflow-hidden">
      <div className="flex items-center justify-between border-b border-gray-100 px-5 py-4">
        <div>
          <h2>Abfrage · {set.title}</h2>
          <p className="mt-1 text-xs text-gray-500">Karte {quiz.index + 1} von {total} · Box {current.box}/5</p>
        </div>
        <button className="text-button" onClick={onExit}>Beenden</button>
      </div>
      <div className="p-5">
        <div className="mb-2 h-1.5 overflow-hidden rounded-full bg-gray-100">
          <div className="h-full rounded-full bg-indigo-600 transition-all" style={{ width: `${(quiz.index / total) * 100}%` }} />
        </div>
        <button
          onClick={() => onAdvance({ ...quiz, revealed: !quiz.revealed })}
          className="grid min-h-56 w-full place-items-center rounded-xl border border-gray-200 bg-gray-50 p-8 text-center transition-colors hover:border-indigo-300"
        >
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-gray-400">{questionLang}</p>
            <p className="mt-2 text-3xl font-bold text-gray-900">{question}</p>
            {current.hint && !quiz.revealed && <p className="mt-3 text-sm text-gray-500">Hinweis: {current.hint}</p>}
            {quiz.revealed ? (
              <>
                <p className="mt-4 text-xs font-bold uppercase tracking-wider text-indigo-500">{answerLang}</p>
                <p className="mt-1 text-2xl font-semibold text-indigo-700">{answer}</p>
                <p className="mt-3 text-xs text-gray-400">Zum Zudecken erneut tippen</p>
              </>
            ) : (
              <p className="mt-4 text-sm font-medium text-indigo-600">Tippen zum Aufdecken</p>
            )}
          </div>
        </button>
        <div className="mt-4 flex gap-2">
          <Button
            className="flex-1"
            variant="secondary"
            disabled={!quiz.revealed || grading}
            loading={grading}
            onClick={() => grade(false)}
          >
            Nicht gewusst
          </Button>
          <Button
            className="flex-1"
            disabled={!quiz.revealed || grading}
            loading={grading}
            onClick={() => grade(true)}
          >
            Gewusst ✓
          </Button>
        </div>
        {!quiz.revealed && <p className="mt-2 text-center text-xs text-gray-400">Decke zuerst die Antwort auf, dann bewerte dich ehrlich.</p>}
      </div>
    </section>
  );
}

function CreateSetDialog({
  pending,
  error,
  onClose,
  onSubmit,
}: {
  pending: boolean;
  error: boolean;
  onClose: () => void;
  onSubmit: (data: CreateVocabSetInput) => Promise<void>;
}) {
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const [form, setForm] = useState({ title: "", description: "", source_lang: "de", target_lang: "en", class_id: "" });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <form
        className="modal"
        onSubmit={async (e) => {
          e.preventDefault();
          setSaving(true);
          setSaveError(false);
          try {
            await onSubmit({
              title: form.title.trim(),
              description: form.description.trim() || undefined,
              source_lang: form.source_lang,
              target_lang: form.target_lang,
              class_id: form.class_id ? Number(form.class_id) : undefined,
            });
          } catch {
            setSaveError(true);
          } finally {
            setSaving(false);
          }
        }}
      >
        <h2>Vokabelset anlegen</h2>
        <div className="form-grid">
          <label className="full">
            Titel
            <input className="mt-1 w-full rounded-lg border border-gray-300 p-2" required value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} placeholder="z. B. Englisch · Unit 3" />
          </label>
          <label className="full">
            Beschreibung
            <input className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} placeholder="Optional" />
          </label>
          <label>
            Von
            <select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.source_lang} onChange={(e) => setForm({ ...form, source_lang: e.target.value })}>
              {LANGS.map((l) => <option key={l.value} value={l.value}>{l.label}</option>)}
            </select>
          </label>
          <label>
            Nach
            <select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.target_lang} onChange={(e) => setForm({ ...form, target_lang: e.target.value })}>
              {LANGS.map((l) => <option key={l.value} value={l.value}>{l.label}</option>)}
            </select>
          </label>
          <label className="full">
            Mit Klasse teilen (optional)
            <select className="mt-1 w-full rounded-lg border border-gray-300 p-2" value={form.class_id} onChange={(e) => setForm({ ...form, class_id: e.target.value })}>
              <option value="">Nur für mich</option>
              {(classes.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </label>
        </div>
        {(error || saveError) && <p className="mt-3 text-sm text-red-600">Das Set konnte nicht angelegt werden.</p>}
        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>Abbrechen</Button>
          <Button loading={pending || saving}>Anlegen</Button>
        </div>
      </form>
    </div>
  );
}
