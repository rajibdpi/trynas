"use client";

import { useState, type ReactNode } from "react";

export function parseCSV(text: string, delimiter = ","): string[][] {
  const rows: string[][] = [];
  let row: string[] = [], cell = "", quoted = false;
  text = text.replace(/^\uFEFF/, "");
  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (char === '"') {
      if (quoted && text[i + 1] === '"') { cell += '"'; i++; }
      else if (quoted || cell === "") quoted = !quoted;
      else cell += char;
    } else if (char === delimiter && !quoted) { row.push(cell); cell = ""; }
    else if ((char === '\n' || char === '\r') && !quoted) {
      if (char === '\r' && text[i + 1] === '\n') i++;
      row.push(cell); rows.push(row); row = []; cell = "";
      if (rows.length >= 1000) return rows;
    } else cell += char;
  }
  if (cell || row.length) { row.push(cell); rows.push(row); }
  return rows;
}

export function CSVViewer({ text, delimiter = "," }: { text: string; delimiter?: string }) {
  const rows = parseCSV(text, delimiter);
  return <div className="max-h-[65vh] overflow-auto rounded border"><table className="w-full text-left text-sm"><tbody>{rows.map((row, index) => <tr key={index} className="border-b">{row.slice(0, 100).map((cell, column) => index === 0 ? <th key={column} className="sticky top-0 whitespace-pre-wrap bg-muted px-3 py-2 font-semibold">{cell}</th> : <td key={column} className="whitespace-pre-wrap px-3 py-2">{cell}</td>)}</tr>)}</tbody></table><p className="p-3 text-xs text-muted-foreground">Preview displays up to 1,000 rows and 100 columns.</p></div>;
}

function inline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\([^)]+\))/g).map((part, index) => {
    if (part.startsWith("**")) return <strong key={index}>{part.slice(2, -2)}</strong>;
    if (part.startsWith("`")) return <code key={index} className="rounded bg-muted px-1">{part.slice(1, -1)}</code>;
    const link = part.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
    if (link && /^https?:\/\//i.test(link[2])) return <a key={index} href={link[2]} target="_blank" rel="noopener noreferrer" className="text-primary underline">{link[1]}</a>;
    return part;
  });
}

export function MarkdownViewer({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  const lines = text.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line.startsWith("```")) {
      const code: string[] = [];
      while (++i < lines.length && !lines[i].startsWith("```")) code.push(lines[i]);
      blocks.push(<pre key={i} className="overflow-auto rounded bg-muted p-3 text-sm"><code>{code.join("\n")}</code></pre>);
    } else if (/^#{1,6} /.test(line)) {
      const level = line.match(/^#+/)![0].length;
      blocks.push(<div role="heading" aria-level={level} key={i} className={level === 1 ? "text-2xl font-bold" : "text-lg font-semibold"}>{inline(line.replace(/^#+ /, ""))}</div>);
    } else if (/^[-*+] /.test(line)) {
      blocks.push(<ul key={i} className="list-disc pl-6"><li>{inline(line.slice(2))}</li></ul>);
    } else if (line.startsWith("> ")) {
      blocks.push(<blockquote key={i} className="border-l-4 pl-3 text-muted-foreground">{inline(line.slice(2))}</blockquote>);
    } else if (line.trim()) blocks.push(<p key={i} className="whitespace-pre-wrap">{inline(line)}</p>);
  }
  return <article className="max-h-[65vh] space-y-3 overflow-auto rounded border p-5 text-sm break-words">{blocks}</article>;
}

export function TextViewer({ text, json = false }: { text: string; json?: boolean }) {
  const [wrap, setWrap] = useState(false);
  let displayed = text, invalidJSON = false;
  if (json) {
    try { displayed = JSON.stringify(JSON.parse(text), null, 2); }
    catch { invalidJSON = true; }
  }
  const lines = displayed.split(/\r?\n/);
  return <div className="space-y-2"><label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={wrap} onChange={event => setWrap(event.target.checked)} />Wrap lines</label>{invalidJSON && <p className="text-xs text-muted-foreground">Invalid JSON; showing the original text.</p>}<div className="max-h-[65vh] overflow-auto rounded border font-mono text-xs"><table className="w-full"><tbody>{lines.slice(0, 10000).map((line, index) => <tr key={index}><td className="w-10 select-none border-r bg-muted px-2 text-right align-top text-muted-foreground">{index + 1}</td><td className={wrap ? "whitespace-pre-wrap break-all px-3" : "whitespace-pre px-3"}>{line || " "}</td></tr>)}</tbody></table></div>{lines.length > 10000 && <p className="text-xs text-muted-foreground">Showing the first 10,000 lines. Download for the complete file.</p>}</div>;
}
