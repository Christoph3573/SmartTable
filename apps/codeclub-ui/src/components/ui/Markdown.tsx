import type { ReactNode } from "react";

/**
 * Minimaler, abhängigkeitsfreier Markdown-Renderer für den KI-Chat.
 *
 * Unterstützt genau das, was die Lern-KI ausgibt: Absätze, Zeilenumbrüche,
 * **fett**, *kursiv*, `Inline-Code`, Codeblöcke, Überschriften, Aufzählungen
 * (sortiert/unsortiert), Blockzitate, Trennlinien und Links.
 *
 * Sicherheit: Es wird NIE `dangerouslySetInnerHTML` verwendet — HTML aus der
 * Modell-Antwort wird als reiner Text gerendert (React escaped von sich aus).
 * Links sind auf http(s)/mailto begrenzt.
 */
const INLINE_PATTERN = /(`[^`]+`|\*\*[^*]+\*\*|__[^_]+__|\*[^*]+\*|_[^_]+_|\[[^\]]+\]\([^)]+\))/g;

function safeHref(raw: string): string | null {
  const url = raw.trim();
  return /^(https?:|mailto:)/i.test(url) ? url : null;
}

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let index = 0;
  INLINE_PATTERN.lastIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = INLINE_PATTERN.exec(text)) !== null) {
    if (match.index > last) out.push(text.slice(last, match.index));
    const token = match[0];
    const key = `${keyPrefix}-${index++}`;
    if (token.startsWith("`")) {
      out.push(
        <code key={key} className="rounded bg-gray-100 px-1 py-0.5 font-mono text-[.8em] text-gray-700">
          {token.slice(1, -1)}
        </code>
      );
    } else if (token.startsWith("**") || token.startsWith("__")) {
      out.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("[")) {
      const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(token);
      const href = link ? safeHref(link[2]) : null;
      out.push(
        link && href ? (
          <a key={key} className="text-indigo-600 underline" href={href} target="_blank" rel="noreferrer">
            {link[1]}
          </a>
        ) : (
          token
        )
      );
    } else if (token.startsWith("*") || token.startsWith("_")) {
      out.push(<em key={key}>{token.slice(1, -1)}</em>);
    } else {
      out.push(token);
    }
    last = match.index + token.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

const LIST_RE = { bullet: /^\s*[-*+]\s+/, ordered: /^\s*\d+[.)]\s+/ };
const HEADING_RE = /^(#{1,6})\s+(.*)$/;
const CODE_FENCE_RE = /^```/;
const BLOCK_START_RE = /^(#{1,6}\s+|>\s?|```)/;

function renderBlocks(content: string): ReactNode[] {
  const lines = content.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let i = 0;
  let key = 0;
  while (i < lines.length) {
    const line = lines[i];

    if (CODE_FENCE_RE.test(line.trim())) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !CODE_FENCE_RE.test(lines[i].trim())) code.push(lines[i++]);
      i++;
      blocks.push(
        <pre key={key++} className="my-2 overflow-x-auto rounded-lg bg-gray-900 p-3 text-xs text-gray-100">
          <code>{code.join("\n")}</code>
        </pre>
      );
      continue;
    }

    if (line.trim() === "") {
      i++;
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      const level = heading[1].length;
      blocks.push(
        <p key={key++} className={`mt-3 mb-1 font-semibold ${level <= 2 ? "text-base" : "text-sm"}`}>
          {renderInline(heading[2], `h${key}`)}
        </p>
      );
      i++;
      continue;
    }

    if (/^(-{3,}|\*{3,}|_{3,})$/.test(line.trim())) {
      blocks.push(<hr key={key++} className="my-3 border-gray-200" />);
      i++;
      continue;
    }

    if (/^>\s?/.test(line)) {
      const quote: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) quote.push(lines[i++].replace(/^>\s?/, ""));
      blocks.push(
        <blockquote key={key++} className="my-2 border-l-4 border-gray-300 pl-3 text-gray-600">
          {renderInline(quote.join(" "), `q${key}`)}
        </blockquote>
      );
      continue;
    }

    const ordered = LIST_RE.ordered.test(line);
    if (ordered || LIST_RE.bullet.test(line)) {
      const listRe = ordered ? LIST_RE.ordered : LIST_RE.bullet;
      const items: string[] = [];
      while (i < lines.length && listRe.test(lines[i])) items.push(lines[i++].replace(listRe, ""));
      const children = items.map((item, idx) => <li key={idx}>{renderInline(item, `li${key}-${idx}`)}</li>);
      const className = `my-2 space-y-1 pl-5 ${ordered ? "list-decimal" : "list-disc"}`;
      blocks.push(ordered ? <ol key={key++} className={className}>{children}</ol> : <ul key={key++} className={className}>{children}</ul>);
      continue;
    }

    const paragraph: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !BLOCK_START_RE.test(lines[i]) &&
      !LIST_RE.bullet.test(lines[i]) &&
      !LIST_RE.ordered.test(lines[i])
    ) {
      paragraph.push(lines[i++]);
    }
    blocks.push(
      <p key={key++} className="my-1 whitespace-pre-wrap break-words">
        {renderInline(paragraph.join("\n"), `p${key}`)}
      </p>
    );
  }
  return blocks;
}

export function Markdown({ content, className = "" }: { content: string; className?: string }) {
  return <div className={className}>{renderBlocks(content)}</div>;
}
