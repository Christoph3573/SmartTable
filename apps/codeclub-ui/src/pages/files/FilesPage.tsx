import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { schoolApi } from "../../api/school";
import { useAuthStore } from "../../store/authStore";
import { Button } from "../../components/ui/Button";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "../../components/ui/Page";
import { formatDate } from "../../lib/format";

const size = (bytes: number) => bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / (1024 * 1024)).toFixed(1)} MB`;

export function FilesPage() {
  const [classId, setClassId] = useState<number>(); const input = useRef<HTMLInputElement>(null); const client = useQueryClient(); const user = useAuthStore((state) => state.user);
  const classes = useQuery({ queryKey: ["classes"], queryFn: schoolApi.classes });
  const activeClassId = classId ?? classes.data?.[0]?.id;
  const folders = useQuery({ queryKey: ["folders", activeClassId], queryFn: () => schoolApi.folders(activeClassId!), enabled: Boolean(activeClassId) });
  const files = useQuery({ queryKey: ["files", activeClassId], queryFn: () => schoolApi.files(activeClassId!), enabled: Boolean(activeClassId) });
  const upload = useMutation({ mutationFn: (file: File) => schoolApi.uploadFile(activeClassId!, file), onSuccess: () => client.invalidateQueries({ queryKey: ["files", activeClassId] }) });
  const remove = useMutation({ mutationFn: schoolApi.deleteFile, onSuccess: () => client.invalidateQueries({ queryKey: ["files", activeClassId] }) });
  const canUpload = user?.role !== "student";
  const download = async (id: number, name: string) => { const response = await schoolApi.downloadFile(id); const url = URL.createObjectURL(response.data as Blob); const link = document.createElement("a"); link.href = url; link.download = name; link.click(); URL.revokeObjectURL(url); };
  return <div className="page"><PageHeader eyebrow="Materialien" title="Dateien"><select aria-label="Klasse wählen" className="rounded-lg border border-[#dfe1da] bg-white px-3 py-2 text-sm" value={activeClassId ?? ""} onChange={(event) => setClassId(Number(event.target.value))}>{classes.data?.map((schoolClass) => <option value={schoolClass.id} key={schoolClass.id}>{schoolClass.name}</option>)}</select>{canUpload && <><input ref={input} className="hidden" type="file" onChange={(event) => { const file = event.target.files?.[0]; if (file) upload.mutate(file); event.target.value = ""; }} /><Button loading={upload.isPending} onClick={() => input.current?.click()}>Datei hochladen</Button></>}</PageHeader>
    {classes.isLoading || (activeClassId && files.isLoading) ? <LoadingState /> : classes.isError || files.isError ? <ErrorState onRetry={() => { classes.refetch(); files.refetch(); }} /> : !activeClassId ? <EmptyState title="Keine Klasse verfügbar" description="Sobald du einer Klasse zugeordnet bist, erscheinen die Materialien hier." /> : <div className="grid gap-5 lg:grid-cols-[.7fr_1.3fr]"><section className="surface overflow-hidden"><div className="border-b border-[#eeeee8] px-5 py-4"><h2>Ordner</h2></div><div className="data-row"><span className="text-lg">▣</span><div className="data-row-main"><strong>Alle Materialien</strong><p>{files.data?.length ?? 0} Dateien</p></div></div>{folders.isLoading ? <LoadingState label="Ordner werden geladen" /> : folders.data?.map((folder) => <div className="data-row" key={folder.id}><span className="text-lg">□</span><div className="data-row-main"><strong>{folder.name}</strong><p>Ordner</p></div></div>)}</section><section className="surface overflow-hidden"><div className="flex items-center justify-between border-b border-[#eeeee8] px-5 py-4"><h2>Alle Materialien</h2><span className="text-xs text-[#777c72]">{files.data?.length ?? 0} Einträge</span></div>{files.data?.length ? files.data.map((file) => <div className="data-row" key={file.id}><span className="grid size-9 place-items-center rounded-lg bg-[#eef1ec] font-mono text-xs text-[#58705b]">{file.name.split(".").pop()?.toUpperCase().slice(0,4) || "FILE"}</span><div className="data-row-main"><strong>{file.name}</strong><p>{size(file.size)} · {file.created_at ? formatDate(file.created_at) : "ohne Datum"}</p></div><button className="text-button" onClick={() => download(file.id, file.name)}>Laden</button>{canUpload && <button className="text-button text-red-600" onClick={() => remove.mutate(file.id)}>Löschen</button>}</div>) : <EmptyState title="Noch keine Materialien" description={canUpload ? "Lade die erste Datei für diese Klasse hoch." : "Hier erscheinen Materialien, die deine Lehrkräfte teilen."} />}</section></div>}
    {upload.isError && <p className="mt-3 text-sm text-red-600">Die Datei konnte nicht hochgeladen werden.</p>}
  </div>;
}
