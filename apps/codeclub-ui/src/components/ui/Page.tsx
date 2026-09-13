import type { ReactNode } from "react";

export function PageHeader({ eyebrow, title, children }: { eyebrow?: string; title: string; children?: ReactNode }) {
  return <header className="page-header"><div><p className="eyebrow">{eyebrow ?? "SmartTable"}</p><h1>{title}</h1></div>{children && <div className="header-actions">{children}</div>}</header>;
}

export function LoadingState({ label = "Daten werden geladen" }: { label?: string }) {
  return <div className="state-card" role="status"><span className="loader" />{label}</div>;
}

export function ErrorState({ onRetry, message = "Die Daten konnten nicht geladen werden." }: { onRetry: () => void; message?: string }) {
  return <div className="state-card state-error"><div><strong>Verbindung unterbrochen</strong><p>{message}</p></div><button className="text-button" onClick={onRetry}>Erneut versuchen</button></div>;
}

export function EmptyState({ title, description, children }: { title: string; description: string; children?: ReactNode }) {
  return <div className="empty-state"><span aria-hidden="true">✦</span><strong>{title}</strong><p>{description}</p>{children}</div>;
}
