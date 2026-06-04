"use client";

import Link from "next/link";
import { cn } from "@/lib/utils";

function renderInline(text: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  const re = /(\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\)|`[^`]+`)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) parts.push(text.slice(last, m.index));
    const token = m[0];
    if (token.startsWith("**")) {
      parts.push(<strong key={m.index}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("`")) {
      parts.push(
        <code
          key={m.index}
          className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]"
        >
          {token.slice(1, -1)}
        </code>,
      );
    } else {
      const linkMatch = /\[([^\]]+)\]\(([^)]+)\)/.exec(token);
      if (linkMatch) {
        const [, label, href] = linkMatch;
        const external = href.startsWith("http");
        if (external) {
          parts.push(
            <a
              key={m.index}
              href={href}
              className="text-primary underline-offset-4 hover:underline"
              target="_blank"
              rel="noreferrer"
            >
              {label}
            </a>,
          );
        } else {
          parts.push(
            <Link
              key={m.index}
              href={href}
              className="text-primary underline-offset-4 hover:underline"
            >
              {label}
            </Link>,
          );
        }
      }
    }
    last = m.index + token.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

export function MarkdownContent({
  source,
  className,
}: {
  source: string;
  className?: string;
}) {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const nodes: React.ReactNode[] = [];
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.startsWith("```")) {
      const lang = line.slice(3).trim();
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith("```")) {
        code.push(lines[i]);
        i++;
      }
      i++;
      nodes.push(
        <pre
          key={key++}
          className="overflow-x-auto rounded-lg border bg-muted/50 p-4 text-xs leading-relaxed"
        >
          <code className={lang ? `language-${lang}` : undefined}>
            {code.join("\n")}
          </code>
        </pre>,
      );
      continue;
    }

    if (/^\|.+\|$/.test(line.trim())) {
      const tableRows: string[][] = [];
      while (i < lines.length && /^\|.+\|$/.test(lines[i].trim())) {
        if (!/^\|[\s\-:|]+\|$/.test(lines[i].trim())) {
          tableRows.push(
            lines[i]
              .trim()
              .slice(1, -1)
              .split("|")
              .map((c) => c.trim()),
          );
        }
        i++;
      }
      if (tableRows.length > 0) {
        const [head, ...body] = tableRows;
        nodes.push(
          <div key={key++} className="overflow-x-auto">
            <table className="w-full border-collapse text-sm">
              <thead>
                <tr className="border-b bg-muted/40">
                  {head.map((cell, ci) => (
                    <th key={ci} className="px-3 py-2 text-left font-medium">
                      {renderInline(cell)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {body.map((row, ri) => (
                  <tr key={ri} className="border-b border-border/60">
                    {row.map((cell, ci) => (
                      <td key={ci} className="px-3 py-2 align-top">
                        {renderInline(cell)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>,
        );
      }
      continue;
    }

    if (line.startsWith("### ")) {
      nodes.push(
        <h3 key={key++} className="mt-6 text-base font-semibold">
          {renderInline(line.slice(4))}
        </h3>,
      );
      i++;
      continue;
    }
    if (line.startsWith("## ")) {
      nodes.push(
        <h2 key={key++} className="mt-8 border-b pb-2 text-lg font-semibold">
          {renderInline(line.slice(3))}
        </h2>,
      );
      i++;
      continue;
    }
    if (line.startsWith("# ")) {
      nodes.push(
        <h1 key={key++} className="text-2xl font-bold tracking-tight">
          {renderInline(line.slice(2))}
        </h1>,
      );
      i++;
      continue;
    }

    if (/^-\s/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^-\s/.test(lines[i])) {
        items.push(lines[i].replace(/^-\s/, ""));
        i++;
      }
      nodes.push(
        <ul key={key++} className="my-3 list-disc space-y-1 pl-6 text-sm">
          {items.map((item, idx) => (
            <li key={idx}>{renderInline(item)}</li>
          ))}
        </ul>,
      );
      continue;
    }

    if (/^\d+\.\s/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s/.test(lines[i])) {
        items.push(lines[i].replace(/^\d+\.\s/, ""));
        i++;
      }
      nodes.push(
        <ol key={key++} className="my-3 list-decimal space-y-1 pl-6 text-sm">
          {items.map((item, idx) => (
            <li key={idx}>{renderInline(item)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    if (line.trim() === "") {
      i++;
      continue;
    }

    nodes.push(
      <p key={key++} className="my-3 text-sm leading-relaxed text-foreground/90">
        {renderInline(line)}
      </p>,
    );
    i++;
  }

  return (
    <article className={cn("max-w-3xl space-y-1", className)}>{nodes}</article>
  );
}
